package main

type Interceptor interface {
	OnMessage(msg *AnthropicMessage)
	OnDeltaStart(delta *OpenAIDelta)
}

type InterceptorFactory interface {
	ShouldIntercept(backendURL string) bool
	Create() Interceptor
}

var interceptorFactories []InterceptorFactory
var interceptor Interceptor

func RegisterInterceptorFactory(factory InterceptorFactory) {
	interceptorFactories = append(interceptorFactories, factory)
}

func InitInterceptor() {
	for _, factory := range interceptorFactories {
		if factory.ShouldIntercept(backendURL) {
			interceptor = factory.Create()
			return
		}
	}
}

func CreateStreamInterceptor() Interceptor {
	for _, factory := range interceptorFactories {
		if factory.ShouldIntercept(backendURL) {
			return factory.Create()
		}
	}
	return nil
}
