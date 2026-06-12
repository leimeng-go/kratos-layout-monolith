package data

import "fmt"

const (
	userCacheIdPrefix       = "cache:users:id:"
	userCacheUsernamePrefix = "cache:users:username:"
	userCacheEmailPrefix    = "cache:users:email:"
)

type userUniqueIndex struct {
	column string
	key    func(string) string
	value  func(*User) string
}

var userUniqueIndexes = []userUniqueIndex{
	{column: "username", key: userCacheUsernameKey, value: func(data *User) string { return data.Username }},
	{column: "email", key: userCacheEmailKey, value: func(data *User) string { return data.Email }},
}

func userCacheIdKey(id int64) string {
	return fmt.Sprintf("%s%d", userCacheIdPrefix, id)
}

func userCacheUsernameKey(username string) string {
	return fmt.Sprintf("%s%s", userCacheUsernamePrefix, username)
}

func userCacheEmailKey(email string) string {
	return fmt.Sprintf("%s%s", userCacheEmailPrefix, email)
}

func (r *userRepo) modelCacheKeys(data *User) []string {
	keys := make([]string, 0, 1+len(userUniqueIndexes))
	if data.Id > 0 {
		keys = append(keys, userCacheIdKey(data.Id))
	}
	keys = append(keys, r.modelUniqueCacheKeys(data)...)
	return keys
}

func (r *userRepo) modelUniqueCacheKeys(data *User) []string {
	keys := make([]string, 0, len(userUniqueIndexes))
	for _, index := range userUniqueIndexes {
		value := index.value(data)
		if value != "" {
			keys = append(keys, index.key(value))
		}
	}
	return keys
}
