package etcd

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.etcd.io/etcd/client/v3"
)

// ServiceRegister 服务注册结构体
type ServiceRegister struct {
	client        *clientv3.Client
	serviceName   string
	serviceAddr   string
	serviceTTL    int64
	leaseID       clientv3.LeaseID
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
}

// NewServiceRegister 创建服务注册实例
func NewServiceRegister(endpoints []string, serviceName, serviceAddr string, serviceTTL int64) (*ServiceRegister, error) {
	// 创建etcd客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &ServiceRegister{
		client:      client,
		serviceName: serviceName,
		serviceAddr: serviceAddr,
		serviceTTL:  serviceTTL,
	}, nil
}

// Register 注册服务
func (r *ServiceRegister) Register() error {
	// 创建租约
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建租约
	resp, err := r.client.Grant(ctx, r.serviceTTL)
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}

	r.leaseID = resp.ID

	// 注册服务
	serviceKey := fmt.Sprintf("/services/%s/%s", r.serviceName, r.serviceAddr)
	serviceValue := r.serviceAddr

	_, err = r.client.Put(ctx, serviceKey, serviceValue, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("failed to put service key: %w", err)
	}

	// 启动租约续期
	r.keepAliveChan, err = r.client.KeepAlive(context.Background(), r.leaseID)
	if err != nil {
		return fmt.Errorf("failed to start keepalive: %w", err)
	}

	// 处理租约续期响应
	go r.handleKeepAlive()

	log.Printf("Service %s registered at %s with lease ID %d", r.serviceName, r.serviceAddr, r.leaseID)
	return nil
}

// handleKeepAlive 处理租约续期
func (r *ServiceRegister) handleKeepAlive() {
	for range r.keepAliveChan {
		// 正常续期，无需处理
	}
	log.Printf("Keepalive channel closed for service %s", r.serviceName)
}

// Unregister 注销服务
func (r *ServiceRegister) Unregister() error {
	if r.leaseID != 0 {
		_, err := r.client.Revoke(context.Background(), r.leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
		log.Printf("Service %s unregistered", r.serviceName)
	}

	if r.client != nil {
		r.client.Close()
	}

	return nil
}
