package cache

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/gorm"
)

type txKey struct{}

const cacheSafeGapBetweenIndexAndPrimary = 5 * time.Second

var (
	CachedDBProviderSet = wire.NewSet(NewCachedDB)
	fieldNamePattern    = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

type CachedDB struct {
	db     *gorm.DB
	cache  *Redis
	logger *log.Helper
}

func NewCachedDB(db *gorm.DB, rds *Redis, logger log.Logger) *CachedDB {
	return &CachedDB{
		db:     db,
		cache:  rds,
		logger: log.NewHelper(log.With(logger, "component", "cached_db")),
	}
}

func (c *CachedDB) DB() *gorm.DB {
	return c.db
}

func (c *CachedDB) DBCtx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return c.db.WithContext(ctx)
}

func (c *CachedDB) Trans(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func (c *CachedDB) QueryRow(ctx context.Context, dest any, cacheKey string, queryFn func() error) error {
	return c.cache.Take(ctx, cacheKey, dest, queryFn)
}

func (c *CachedDB) QueryRowIndex(ctx context.Context, dest any, indexKey string,
	indexQuery func() (primaryCacheKey string, err error),
	primaryQuery func() error,
) error {
	var primaryCacheKey string
	var fresh bool

	err := c.cacheTakeWithIndex(ctx, dest, indexKey, &primaryCacheKey, &fresh, indexQuery)
	if err != nil {
		return err
	}

	if !fresh {
		return c.cache.Take(ctx, primaryCacheKey, dest, primaryQuery)
	}

	return nil
}

func (c *CachedDB) cacheTakeWithIndex(ctx context.Context, dest any, indexKey string,
	primaryCacheKey *string, fresh *bool,
	indexQuery func() (string, error),
) error {
	c.cache.collector.Miss(indexKey)

	data, err := c.cache.cacheGet(ctx, indexKey)
	if err == nil && len(data) > 0 {
		if string(data) == notFoundPlaceholder {
			c.cache.collector.Hit(indexKey)
			return ErrNotFound
		}
		c.cache.collector.Hit(indexKey)
		*primaryCacheKey = string(data)
		*fresh = false
		return nil
	}

	type indexResult struct {
		primaryCacheKey string
		nf              bool
	}

	val2, err, _ := c.cache.sf.Do(indexKey, func() (any, error) {
		c.cache.collector.DBOp(indexKey)
		c.cache.collector.SingleFlight(indexKey)

		pk, qerr := indexQuery()
		if errors.Is(qerr, ErrNotFound) {
			c.cache.cacheSetNotFound(ctx, indexKey)
			return indexResult{nf: true}, nil
		}
		if qerr != nil {
			c.cache.collector.DbFail(indexKey)
			return nil, qerr
		}

		c.cache.cacheSet(ctx, indexKey, []byte(pk), c.cache.Expiry())

		rowData, _ := json.Marshal(dest)
		c.cache.cacheSet(ctx, pk, rowData, c.cache.Expiry()+cacheSafeGapBetweenIndexAndPrimary)

		return indexResult{primaryCacheKey: pk}, nil
	})
	if err != nil {
		return err
	}

	res := val2.(indexResult)
	if res.nf {
		return ErrNotFound
	}

	*primaryCacheKey = res.primaryCacheKey
	*fresh = true
	return nil
}

func (c *CachedDB) Exec(ctx context.Context, execFn func() error, cacheKeys ...string) error {
	if err := execFn(); err != nil {
		return err
	}

	if len(cacheKeys) > 0 {
		if err := c.cache.Del(ctx, cacheKeys...); err != nil {
			AsyncDel(context.Background(), c.cache, 3, []time.Duration{0, time.Second, 5 * time.Second}, cacheKeys...)
		}
	}

	return nil
}

func (c *CachedDB) QueryRowNoCache(ctx context.Context, queryFn func() error) error {
	return queryFn()
}

func (c *CachedDB) QueryRowsNoCache(ctx context.Context, queryFn func() error) error {
	return queryFn()
}

func (c *CachedDB) FindCount(ctx context.Context, model any, build func(*gorm.DB) *gorm.DB) (int64, error) {
	var count int64
	db := c.DBCtx(ctx).Model(model)
	if build != nil {
		db = build(db)
	}
	return count, db.Count(&count).Error
}

func (c *CachedDB) FindSum(ctx context.Context, model any, field string, build func(*gorm.DB) *gorm.DB) (float64, error) {
	if !fieldNamePattern.MatchString(field) {
		return 0, errors.New("invalid sum field")
	}
	var sum float64
	db := c.DBCtx(ctx).Model(model)
	if build != nil {
		db = build(db)
	}
	return sum, db.Select("COALESCE(SUM(" + field + "), 0)").Scan(&sum).Error
}

func (c *CachedDB) FindAll(ctx context.Context, dest any, build func(*gorm.DB) *gorm.DB, orderBy string) error {
	db := c.DBCtx(ctx)
	if build != nil {
		db = build(db)
	}
	if orderBy != "" {
		db = db.Order(orderBy)
	}
	return db.Find(dest).Error
}

func (c *CachedDB) FindPageListByPage(ctx context.Context, dest any, build func(*gorm.DB) *gorm.DB, page, pageSize int32, orderBy string) error {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	db := c.DBCtx(ctx)
	if build != nil {
		db = build(db)
	}
	if orderBy != "" {
		db = db.Order(orderBy)
	}
	return db.Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(dest).Error
}

func (c *CachedDB) FindPageListByPageWithTotal(ctx context.Context, dest any, model any, build func(*gorm.DB) *gorm.DB, page, pageSize int32, orderBy string) (int64, error) {
	total, err := c.FindCount(ctx, model, build)
	if err != nil {
		return 0, err
	}
	return total, c.FindPageListByPage(ctx, dest, build, page, pageSize, orderBy)
}

func (c *CachedDB) FindPageListByIdDesc(ctx context.Context, dest any, build func(*gorm.DB) *gorm.DB, lastId, pageSize int32) error {
	if pageSize <= 0 {
		pageSize = 10
	}
	db := c.DBCtx(ctx)
	if build != nil {
		db = build(db)
	}
	if lastId > 0 {
		db = db.Where("id < ?", lastId)
	}
	return db.Order("id DESC").Limit(int(pageSize)).Find(dest).Error
}

func (c *CachedDB) FindPageListByIdAsc(ctx context.Context, dest any, build func(*gorm.DB) *gorm.DB, lastId, pageSize int32) error {
	if pageSize <= 0 {
		pageSize = 10
	}
	db := c.DBCtx(ctx)
	if build != nil {
		db = build(db)
	}
	if lastId > 0 {
		db = db.Where("id > ?", lastId)
	}
	return db.Order("id ASC").Limit(int(pageSize)).Find(dest).Error
}

func (c *CachedDB) Delete(ctx context.Context, model any, id any, cacheKeys ...string) error {
	return c.Exec(ctx, func() error {
		return c.DBCtx(ctx).Delete(model, id).Error
	}, cacheKeys...)
}

func (c *CachedDB) DelCache(ctx context.Context, keys ...string) error {
	return c.cache.Del(ctx, keys...)
}

func (c *CachedDB) SetCache(ctx context.Context, key string, val any) error {
	return c.cache.Set(ctx, key, val, c.cache.Expiry())
}
