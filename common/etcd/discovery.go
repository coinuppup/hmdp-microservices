package etcd

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/client/v3"
)

// ServiceDiscovery 服务发现结构体
type ServiceDiscovery struct {
	client       *clientv3.Client
	serviceName  string
	serviceAddrs []string
}

// NewServiceDiscovery 创建服务发现实例
func NewServiceDiscovery(endpoints []string, serviceName string) (*ServiceDiscovery, error) {
	// 创建etcd客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	discovery := &ServiceDiscovery{
		client:       client,
		serviceName:  serviceName,
		serviceAddrs: []string{},
	}

	// 初始化服务列表
	if err := discovery.refreshServiceList(); err != nil {
		return nil, err
	}

	// 启动服务变更监听
	go discovery.watchService()

	return discovery, nil
}

// refreshServiceList 刷新服务列表
func (d *ServiceDiscovery) refreshServiceList() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 前缀查询
	servicePrefix := fmt.Sprintf("/services/%s/", d.serviceName)
	resp, err := d.client.Get(ctx, servicePrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get service list: %w", err)
	}

	// 解析服务列表
	addrs := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addrs = append(addrs, string(kv.Value))
	}

	d.serviceAddrs = addrs
	log.Printf("Service %s discovered: %v", d.serviceName, addrs)
	return nil
}

// watchService 监听服务变更
func (d *ServiceDiscovery) watchService() {
	servicePrefix := fmt.Sprintf("/services/%s/", d.serviceName)
	watcher := d.client.Watch(context.Background(), servicePrefix, clientv3.WithPrefix())

	for watchResp := range watcher {
		if watchResp.Err != nil {
			log.Printf("Watch error: %v", watchResp.Err)
			continue
		}

		// 服务发生变更，刷新服务列表
		if err := d.refreshServiceList(); err != nil {
			log.Printf("Failed to refresh service list: %v", err)
		}
	}
}

// GetServiceAddr 获取服务地址
func (d *ServiceDiscovery) GetServiceAddr() (string, error) {
	if len(d.serviceAddrs) == 0 {
		return "", fmt.Errorf("no service found for %s", d.serviceName)
	}

	// 简单轮询负载均衡
	addr := d.serviceAddrs[0]
	// 轮询
	d.serviceAddrs = append(d.serviceAddrs[1:], addr)

	return addr, nil
}

// GetAllServiceAddrs 获取所有服务地址
func (d *ServiceDiscovery) GetAllServiceAddrs() []string {
	return d.serviceAddrs
}

// Close 关闭客户端
func (d *ServiceDiscovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
