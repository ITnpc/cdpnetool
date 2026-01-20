// Package regexutil 提供带并发安全缓存的正则表达式编译工具
package regexutil

import (
	"regexp"
	"sync"
)

// Cache 正则表达式编译器缓存
// 内部使用 sync.Map 优化读多写少的并发场景
type Cache struct {
	cache sync.Map
}

// New 创建一个新的正则缓存实例
func New() *Cache {
	return &Cache{}
}

// Get 获取编译后的正则表达式对象
// 如果缓存中已存在则直接返回，否则进行编译并存入缓存
func (c *Cache) Get(p string) (*regexp.Regexp, error) {
	// 1. 尝试从缓存中读取
	if val, ok := c.cache.Load(p); ok {
		return val.(*regexp.Regexp), nil
	}

	// 2. 编译正则
	compiled, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}

	// 3. 存入缓存
	c.cache.Store(p, compiled)
	return compiled, nil
}
