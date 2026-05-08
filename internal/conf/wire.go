package conf

import "github.com/google/wire"

// ProviderSet extracts config sub-structures from Bootstrap.
var ProviderSet = wire.NewSet(
	ServerFromBootstrap,
	AuthFromBootstrap,
	JwtFromBootstrap,
	DatabaseFromBootstrap,
	RedisFromBootstrap,
)

func ServerFromBootstrap(b *Bootstrap) *Server    { return b.Server }
func AuthFromBootstrap(b *Bootstrap) *Auth        { return b.Auth }
func JwtFromBootstrap(b *Bootstrap) *Jwt          { return b.Jwt }
func DatabaseFromBootstrap(b *Bootstrap) *Database { return b.Data.Database }
func RedisFromBootstrap(b *Bootstrap) *Redis      { return b.Data.Redis }
 