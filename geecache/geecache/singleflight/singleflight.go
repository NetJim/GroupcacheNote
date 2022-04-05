package singleflight

import "sync"

// 防止缓存击穿

// call 代表正在进行中，或已经结束的请求。使用 sync.WaitGroup 锁避免重入。
// Group 是 singleflight 的主数据结构，管理不同 key 的请求(call)。

// 将请求包装为call
// 用group结构体里面的map做一个call集合
// 如果相同的call已经包含在集合里面了，就等待之前的相同的call执行完
// 相同的call只用返回第一个call的结果就行了

// 在一瞬间有大量请求get(key)，而且key未被缓存或者未被缓存在当前节点
// 如果不用singleflight，那么这些请求都会发送远端节点或者从本地数据库读取，
// 会造成远端节点或本地数据库压力猛增。使用singleflight，第一个get(key)请求到来时，
// singleflight会记录当前key正在被处理，后续的请求只需要等待第一个请求处理完成，取返回值即可。
// 并发场景下如果 GeeCache 已经向其他节点/源获取数据了，那么就加锁阻塞其他相同的请求，等待请求结果，防止其他节点/源压力猛增被击穿。

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex // protect m
	m  map[string]*call
}

// Do 方法，接收 2 个参数，第一个参数是 key，第二个参数是一个函数 fn。
// Do 的作用就是，针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误。

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()         // 如果请求正在进行中，则等待
		return c.val, c.err // 请求结束，返回结果
	}
	c := new(call)
	c.wg.Add(1)  // 发起请求前加锁
	g.m[key] = c // 添加到g.m，表明key已经有对应的请求在处理
	g.mu.Unlock()

	c.val, c.err = fn() // 调用fn，发起请求
	c.wg.Done()         // 请求结束

	delete(g.m, key)    // 更新g.m
	return c.val, c.err // 返回结果
}
