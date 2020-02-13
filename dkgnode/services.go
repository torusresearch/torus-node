package dkgnode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/avast/retry-go"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/eventbus"
)

type ServiceRegistry struct {
	eventBus      eventbus.Bus
	middlewareMap sync.Map
	Services      map[string]*BaseService
}

func NewServiceRegistry(eventBus eventbus.Bus, baseServices ...*BaseService) *ServiceRegistry {
	serviceRegistry := &ServiceRegistry{
		Services: make(map[string]*BaseService),
	}
	serviceRegistry.SetEventBus(eventBus)
	serviceRegistry.SetupMethodRouting()
	for _, baseService := range baseServices {
		serviceRegistry.RegisterService(baseService)
	}
	return serviceRegistry
}

func (s *ServiceRegistry) AddMiddleware(middleware *func(string, interface{}) interface{}) {
	s.middlewareMap.Store(middleware, middleware)
	s.eventBus.AddMiddleware(middleware)
}

func (s *ServiceRegistry) RemoveMiddleware(m *func(string, interface{}) interface{}) {
	middleware, ok := s.middlewareMap.Load(m)
	if !ok {
		return
	}
	s.eventBus.RemoveMiddleware(middleware.(*func(string, interface{}) interface{}))
	s.middlewareMap.Delete(middleware)
}

func (s *ServiceRegistry) AddRequestMiddleware(middleware *func(methodRequest MethodRequest) MethodRequest) {
	requestMiddleware := func(topic string, inter interface{}) interface{} {
		if methodRequest, ok := inter.(MethodRequest); ok {
			return (*middleware)(methodRequest)
		}
		return inter
	}
	s.middlewareMap.Store(middleware, &requestMiddleware)
	s.eventBus.AddMiddleware(&requestMiddleware)
}

func (s *ServiceRegistry) AddResponseMiddleware(middleware *func(methodResponse MethodResponse) MethodResponse) {
	responseMiddleware := func(topic string, inter interface{}) interface{} {
		if methodResponse, ok := inter.(MethodResponse); ok {
			return (*middleware)(methodResponse)
		}
		return inter
	}
	s.middlewareMap.Store(middleware, &responseMiddleware)
	s.eventBus.AddMiddleware(&responseMiddleware)
}

func (s *ServiceRegistry) RemoveRequestMiddleware(middleware *func(methodRequest MethodRequest) MethodRequest) {
	requestMiddleware, ok := s.middlewareMap.Load(middleware)
	if !ok {
		return
	}
	s.eventBus.RemoveMiddleware(requestMiddleware.(*func(string, interface{}) interface{}))
	s.middlewareMap.Delete(middleware)
}

func (s *ServiceRegistry) RemoveResponseMiddleware(middleware *func(methodResponse MethodResponse) MethodResponse) {
	responseMiddleware, ok := s.middlewareMap.Load(middleware)
	if !ok {
		return
	}
	s.eventBus.RemoveMiddleware(responseMiddleware.(*func(string, interface{}) interface{}))
	s.middlewareMap.Delete(middleware)
}

func (s *ServiceRegistry) SetEventBus(e eventbus.Bus) {
	s.eventBus = e
}

func (s *ServiceRegistry) RegisterService(bs *BaseService) {
	s.Services[bs.Name()] = bs
}

type MethodRequest struct {
	Caller  string
	Service string
	Method  string
	ID      string
	Data    []interface{}
}

func (s *ServiceRegistry) SetupMethodRouting() {
	err := s.eventBus.SubscribeAsync("method", func(data interface{}) {
		methodRequest, ok := data.(MethodRequest)
		if !ok {
			logging.Error("could not parse data for query")
			return
		}
		var baseService *BaseService
		err := retry.Do(func() error {
			bs, ok := s.Services[methodRequest.Service]
			if !ok {
				logging.WithField("Service", methodRequest.Service).Error("could not find service")
				return fmt.Errorf("Could not find service %v", methodRequest.Service)
			}
			if !bs.IsRunning() {
				logging.WithFields(logging.Fields{
					"Service": methodRequest.Service,
					"data":    data,
				}).Error("Service is not running")
				return fmt.Errorf("Service %v is not running", methodRequest.Service)
			}
			baseService = bs
			return nil
		})
		if err != nil {
			s.eventBus.Publish(methodRequest.ID, MethodResponse{
				Error: err,
				Data:  nil,
			})
		} else {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						logging.WithFields(logging.Fields{
							"Caller": methodRequest.Caller,
							"Method": methodRequest.Method,
							"data":   data,
							"error":  err,
							"Stack":  stringify(debug.Stack()),
						}).Error("panicked during baseService.Call")
						resp := MethodResponse{
							Error: fmt.Errorf("%v", err),
							Data:  nil,
						}
						s.eventBus.Publish(methodRequest.ID, resp)
					}
				}()
				data, err := baseService.Call(methodRequest.Method, methodRequest.Data...)
				resp := MethodResponse{
					Request: methodRequest,
					Error:   err,
					Data:    data,
				}
				s.eventBus.Publish(methodRequest.ID, resp)
			}()
		}
	}, false)
	if err != nil {
		logging.WithError(err).Error("could not subscribe async")
	}
}

type MethodResponse struct {
	Request MethodRequest
	Error   error
	Data    interface{}
}

func AwaitTopic(eventBus eventbus.Bus, topic string) <-chan interface{} {
	responseCh := make(chan interface{})
	err := eventBus.SubscribeOnceAsync(topic, func(res interface{}) {
		responseCh <- res
		close(responseCh)
	})
	if err != nil {
		logging.WithError(err).Error("could not subscribe async")
	}
	return responseCh
}

func ServiceMethod(eventBus eventbus.Bus, caller string, service string, method string, data ...interface{}) MethodResponse {
	nonce, err := rand.Int(rand.Reader, secp256k1.GeneratorOrder)
	if err != nil {
		return MethodResponse{
			Error: errors.New("Could not generate random nonce"),
			Data:  nil,
		}
	}
	nonceStr := nonce.Text(16)
	responseCh := AwaitTopic(eventBus, nonceStr)
	eventBus.Publish("method", MethodRequest{
		Caller:  caller,
		Service: service,
		Method:  method,
		ID:      nonceStr,
		Data:    data,
	})
	methodResponseInter := <-responseCh
	methodResponse, ok := methodResponseInter.(MethodResponse)
	if !ok {
		return MethodResponse{
			Error: errors.New("Method response was not of MethodResponse type"),
			Data:  nil,
		}
	}
	return methodResponse
}

func EmptyHandler(name string) func() {
	return func() {
		logging.WithField("name", name).Error("handler was not initialized")
	}
}
