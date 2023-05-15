# Plugin Development Guide

This documentation introduces how developers can create a plugin.

All plugins must follow these steps:

1. **Create a new plugin module**: Create a new project in the specified directory and import the plugin API module.
2. **Define the enhancement object**: Define the description for the plugin.
3. **Invoke the plugin API**: Call the API provided by the core to complete the core invocation.
4. **Import the plugin module**: Import the plugin into the management module for users to use.

## Create a new plugin module

The plugin must create a new module, which is currently stored in the project's [plugins directory](../../../plugins).

Plugins can import the following two modules:

1. **Agent core**: This module provides all the dependencies needed for the plugin, including the plugin API, enhancement declaration objects, etc.
Agent core plugin should be `github.com/apache/skywalking-go/plugins/core` and replaced by the relative location.
2. **Framework to be enhanced**: Import the framework you wish to enhance.

Note: Plugins should **NOT** import and use any other modules, as this may cause compilation issues for users. If certain tools are needed, they should be provided by the agent core.

## Define the enhancement object

In the root directory of the project, **create a new go file** to define the basic information of the plugin. 
The basic information includes the following methods, corresponding to the [Instrument interface](../../../plugins/core/instrument/declare.go):

1. **Name**: The name of the plugin. Please keep this name consistent with the newly created project name. The reason will be explained later.
2. **Base Package**: Declare which package this plugin intercepts. For example, if you want to intercept gin, you can write: "github.com/gin-gonic/gin".
3. **Version Checker**: This method passes the version number to the enhancement object to verify whether the specified version of the framework is supported. If not, the enhancement program will not be executed.
4. **Points**: A plugin can define one or more enhancement points. This will be explained in more detail in the following sections.
5. **File System**: Use `//go:embed *` in the current file to import all files in this module, which will be used for file copying during the mixed compilation process.

Note: Please declare `//skywalking:nocopy` at any position in this file to indicate that the file would not be copied. This file is only used for guidance during hybrid compilation. 
Also, this file involves the use of the `embed` package, and if the target framework does not import the package `embed`, a compilation error may occur.

### Instrument Point

Instrument points are used to declare that which methods and structs in the current package should be instrumented. They mainly include the following information:

1. **Package path**: If the interception point that needs to be intercepted is not in the root directory of the current package, you need to fill in the relative path to the package. 
For example, if this interception point wants to instrument content in the `github.com/gin-gonic/gin/render` directory, you need to fill in `render` here.
2. **Matcher(At)**: Specify which eligible content in the current package path needs to be enhanced.
3. **Interceptor**: If the current method is being intercepted (whether it's a static method or an instance method), the name of the interceptor must be specified.

#### Method Matcher

Method matchers are used to intercept both static and non-static methods. The specific definitions are as follows:

```go
// NewStaticMethodEnhance creates a new EnhanceMatcher for static method.
// name: method name needs to be enhanced.(Public and private methods are supported)
// filters: filters for method.
func NewStaticMethodEnhance(name string, filters ...MethodFilterOption)

// NewMethodEnhance creates a new EnhanceMatcher for method.
// receiver: receiver type name of method needs to be enhanced.
// name: method name needs to be enhanced.(Public and private methods are supported)
// filters: filters for method.
func NewMethodEnhance(receiver, name string, filters ...MethodFilterOption)
```

##### Filter Option

Filter Options are used to validate the parameters or return values in the method. 
If the method name matches but the Options validation fails, the enhancement would not be performed.

```go
// WithArgsCount filter methods with specific count of arguments. 
func WithArgsCount(argsCount int)

// WithResultCount filter methods with specific count of results.
func WithResultCount(resultCount int)

// WithArgType filter methods with specific type of the index of the argument.
func WithArgType(argIndex int, dataType string)

// WithResultType filter methods with specific type of the index of the result.
func WithResultType(argIndex int, dataType string)
```

##### Demo

For example, if you have the following method that needs to be intercepted:

```go
func (c *Context) HandleMethod(name string) bool
```

you can describe it using this condition:

```go
instrument.NewMethodEnhance("*Context", "HandleMethod", 
	instrument.WithArgsCount(1), instrument.WithArgType(0, "string"), 
	instrument.WithResultCount(1), instrument.WithResultType(0, "bool"))
```

#### Struct Matcher

Enhancement structures can embed enhanced fields within specified structs. 
After the struct is instantiated, custom data content can be added to the specified struct in the method interceptor.

Struct matchers are used to intercept struct methods. The specific definitions are as follows:

```go
// NewStructEnhance creates a new EnhanceMatcher for struct.
// name: struct name needs to be enhanced.(Public and private structs are supported)
// filters: filters for struct.
func NewStructEnhance(name string, filters ...StructFilterOption)
```

##### Filter Option

Filter Options are used to validate the fields in the structure.

```go
// WithFieldExists filter the struct has the field with specific name.
func WithFieldExists(fieldName string)

// WithFiledType filter the struct has the field with specific name and type.
func WithFiledType(filedName, filedType string)
```

##### Enhanced Instance 

After completing the definition of the struct enhancement, you can convert the specified instance into the following interface when intercepting methods, 
and get or set custom field information. The interface definition is as follows:

```go
type EnhancedInstance interface {
	// GetSkyWalkingDynamicField get the customized data from instance
	GetSkyWalkingDynamicField() interface{}
	// SetSkyWalkingDynamicField set the customized data into the instance
	SetSkyWalkingDynamicField(interface{})
}
```

##### Demo

For example, if you have the following struct that needs to be enhanced:

```go
type Test struct {
	value *Context
}
```

you can describe it using this condition:

```go
instrument.NewStructEnhance("Test", instrument.WithFieldExists("value"), instrument.WithFiledType("value", "*Context"))
```

Next, you can set custom content for the specified enhanced instance when intercepting methods.

```go
ins := testInstance.(instrument.EnhancedInstance)
// setting custom content
ins.SetSkyWalkingDynamicField("custom content")
// getting custom content
res := ins.GetSkyWalkingDynamicField()
```

#### Interceptor

Interceptors are used to define custom business logic before and after method execution, 
allowing you to access data from before and after method execution and interact with the Agent Core by using the Agent API.

The interceptor definition is as follows, you need to create a new structure and implement it:

```go
type Interceptor interface {
    // BeforeInvoke would be called before the target method invocation.
    BeforeInvoke(invocation Invocation) error
    // AfterInvoke would be called after the target method invocation.
    AfterInvoke(invocation Invocation, result ...interface{}) error
}
```

Within the interface, you can see the `Invocation` interface, which defines the context of an interception. The specific definition is as follows:

```go
type Invocation interface {
    // CallerInstance is the instance of the caller, nil if the method is static method.
    CallerInstance() interface{}
    // Args is get the arguments of the method, please cast to the specific type to get more information.
    Args() []interface{}

    // ChangeArg is change the argument value of the method
    ChangeArg(int, interface{})

    // IsContinue is the flag to control the method invocation, if it is true, the target method would not be invoked.
    IsContinue() bool
    // DefineReturnValues are defined the return value of the method, and continue the method invoked
    DefineReturnValues(...interface{})

    // SetContext is the customized context of the method invocation, it should be propagated the tracing span.
    SetContext(interface{})
    // GetContext is get the customized context of the method invocation
    GetContext() interface{}
}
```

##### Thread safe

The `Interceptor` instance would **define new instance at the current package level**, 
rather than creating a new instance each time a method is intercepted. 

Therefore, do not declare objects in the interceptor, and instead use `Invocation.Context` to pass data.

##### Package Path

If the method you want to intercept is not located in the root directory of the framework, 
place your interceptor code in the relative location within the plugin. **The Agent would only copy files from the same package directory**.

For example, if you want to intercept a method in `github.com/gin-gonic/gin/render`, create a **render** directory in the root of your plugin, and **put the interceptor inside it**. 
This ensures that the interceptor is properly included during the copy operation and can be correctly applied to the target package.

## Agent API

The Agent API is used when a method is intercepted and interacts with the Agent Core.

### Tracing API

The Tracing API is used for building distributed tracing, and currently supports the following methods:

```go
// CreateEntrySpan creates a new entry span.
// operationName is the name of the span.
// extractor is the extractor to extract the context from the carrier.
// opts is the options to create the span.
func CreateEntrySpan(operationName string, extractor Extractor, opts ...SpanOption)

// CreateLocalSpan creates a new local span.
// operationName is the name of the span.
// opts is the options to create the span.
func CreateLocalSpan(operationName string, opts ...SpanOption)

// CreateExitSpan creates a new exit span.
// operationName is the name of the span.
// peer is the peer address of the span.
// injector is the injector to inject the context into the carrier.
// opts is the options to create the span.
func CreateExitSpan(operationName, peer string, injector Injector, opts ...SpanOption)

// ActiveSpan returns the current active span, it can be got the current span in the current goroutine.
// If the current goroutine is not in the context of the span, it will return nil.
// If get the span from other goroutine, it can only get information but cannot be operated.
func ActiveSpan()

// GetRuntimeContextValue returns the value of the key in the runtime context, which is current goroutine.
// The value can also read from the goroutine which is created by the current goroutine
func GetRuntimeContextValue(key string)

// SetRuntimeContextValue sets the value of the key in the runtime context.
func SetRuntimeContextValue(key string, val interface{})
```

#### Context Carrier

The context carrier is used to pass the context between the difference application.

When creating an Entry Span, you need to obtain the context carrier from the request. 
When creating an Exit Span, you need to write the context carrier into the target RPC request.

```go
// Extractor is a tool specification which define how to
// extract trace parent context from propagation context
type Extractor func(headerKey string) (string, error)

// Injector is a tool specification which define how to
// inject trace context into propagation context
type Injector func(headerKey, headerValue string) error
```

The following demo demonstrates how to pass the Context Carrier in the Tracing API:

```go
// create a new entry span and extract the context carrier from the request
tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), func(headerKey string) (string, error) {
    return request.Header.Get(headerKey), nil
})

// create a new exit span and inject the context carrier into the request
tracing.CreateExitSpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), request.Host, func(headerKey, headerValue string) error {
    request.Header.Add(headerKey, headerValue)
    return nil
}
```

#### Span Option

Span Options can be passed when creating a Span to configure the information in the Span. 

The following options are currently supported:

```go
// WithLayer set the SpanLayer of the Span
func WithLayer(layer SpanLayer)

// WithComponent set the component id of the Span
func WithComponent(componentID int32)

// WithTag set the Tag of the Span
func WithTag(key Tag, value string)
```

##### Span Component

The Component ID in Span is used to identify the current component, with its data [defined in SkyWalking OAP](https://github.com/apache/skywalking/blob/master/oap-server/server-starter/src/main/resources/component-libraries.yml). 
If the framework you are writing does not exist in this file, please submit a PR in the SkyWalking project to add the definition of this plugin.

#### Span Operation

After creating a Span, you can perform additional operations on it.

```go
// Span for plugin API
type Span interface {
	// Tag set the Tag of the Span
	Tag(Tag, string)
	// SetSpanLayer set the SpanLayer of the Span
	SetSpanLayer(SpanLayer)
	// SetOperationName re-set the operation name of the Span
	SetOperationName(string)
	// SetPeer re-set the peer address of the Span
	SetPeer(string)
	// Log add log to the Span
	Log(...string)
	// Error add error log to the Span
	Error(...string)
	// End end the Span
	End()
}
```

## Import Plugin

Once you have finished developing the plugin, you need to import the completed module into the Agent program and [define it in the corresponding file](../../../tools/go-agent/instrument/plugins/register.go). 

At this point, your plugin development process is complete. When the Agent performs hybrid compilation on the target program, your plugin will be executed as expected.