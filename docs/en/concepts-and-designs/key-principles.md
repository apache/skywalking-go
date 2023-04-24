# Key Principle

Enhancing applications in hybrid compilation is very important for SkyWalking Go Agent. 
In this section, I will delve deeper into several key technical points.

## Method Interceptor

Method interception is particularly important in SkyWalking Go, as it enables the creation of plugins. In SkyWalking Go, method interception mainly involves the following key points:

1. **Finding Method**: Using `AST` to find method information in the target code to be enhanced.
2. **Modifying Methods**: Enhancing the specified methods and embedding interceptor code.
3. **Saving and Compiling**: Updating the modified files in the compilation arguments.

### Finding Method

When looking for methods, the SkyWalking Go Agent need to search according to the provided compilation arguments, which mainly include the following two parts:

1. **Package information**: Based on the package name provided by the arguments, the Agent can find the specific plugin.
2 **Go files**: When a matching plugin is found, Agent can read the `.go` files and use `AST` to parse the method information contained in these files. When the method information matches the method information required by the plugin for interception, Agent can consider the method found.

### Modifying Methods

After finding the method, the SkyWalking Go Agent needs to modify the method implication and embed the interceptor code.

#### Change Method Body

When intercepting a method, the first thing to do is to modify the method and [embed the template code](../../../tools/go-agent/instrument/plugins/templates/method_inserts.tmpl). 
This code segment includes two method executions:

1. **Before method execution**: Pass in the current method's arguments, instances, and other information.
2. **After method execution**: Using the `defer` method, intercept the result parameters after the code execution is completed.

Based on these two methods, the agent can intercept before and after method execution. 

In order not to affect the line of code execution, this code segment will only be executed in the **same line as the first statement in the method**. 
This ensures that when an exception occurs in the framework code execution, the exact location can still be found without being affected by the enhanced code.

#### Write Adapter File

After the agent enhances the method body, it needs to implement the above two methods and write them into a single file, called the **adapter file**. These two methods will do the following:

1. **Before method execution**: [Build by the template](../../../tools/go-agent/instrument/plugins/templates/method_intercept_before.tmpl). Build the context for before and after interception, and pass the parameter information during execution to the interceptor in each plugin.
2. **After method execution**: [Build by the template](../../../tools/go-agent/instrument/plugins/templates/method_intercept_after.tmpl). Pass the method return value to the interceptor and execute the method.

#### Copy Files

After completing the adapter file, the agent would perform the following copy operations:

1. **Plugin Code**: Copy the Go files containing the interceptors in the plugin to the same level directory as the current framework.
2. **Plugin Development API Code**: Copy the operation APIs needed by the interceptors in the plugin to the same level directory as the current framework, such as `tracing`.

After copying the files, they cannot be immediately added to the compilation parameters, because they may have the same name as the existing framework code. Therefore, we need to perform some rewriting operations, which include the following parts:

1. **Types**: Rename created structures, interfaces, methods, and other types by adding a unified prefix.
2. **Static Methods**: Add a prefix to non-instance methods. Static methods do not need to be rewritten since they have already been processed in the types.
3. **Variables**: Add a prefix to global variables. It's not necessary to add a prefix to variables inside methods because they can ensure no conflicts would arise and are helpful for debugging.

### Saving and Compiling

After the above steps are completed, the agent needs to save the modified files and add them to the compilation parameters.

At this point, when the framework executes the enhanced method, it can have the following capabilities:

1. **Execute Plugin Code**: Custom code can be embedded before and after the method execution, and real-time parameter information can be obtained.
2. **Operate Agent**: By calling the Agent API, interaction with the Agent Core can be achieved, enabling functions such as distributed tracing.

## Propagation context

In Golang programs, we use `context.Context` to achieve data exchange between methods and goroutines. 
However, if the framework or environment does not provide `context.Context` for propagation, or it is not required as a parameter when used, it would result in the inability to pass information. 
Therefore, we need to consider a method to pass data using **non-**`context.Context` objects to keep the entire distributed tracing chain complete.

### Context Propagation between Methods

In the agent, it would enhance the `g` structure in the `runtime` package. 
The `g` structure in Golang represents the internal data of the current goroutine. 
By enhancing this structure and using the `runtime.getg()` method, we can obtain the enhanced data in the current structure in real-time.

Enhancement includes the following steps:

1. **Add Attributes to g**: Add a new field to the `g` struct, and value as `interface{}`.
2. **Export Methods**: Export methods for real-time setting and getting of custom field values in the current goroutine through `go:linkname`.
3. **Import methods**: In the Agent Core, import the setting and getting methods for custom fields.

After completing the above steps, the agent can get or set data of the same goroutine in any method within the same goroutine, similar to Java's `Thread Local`.

### Context Propagation between Goroutines

For cross-goroutine situations, since different goroutines have different `g` objects, 
the agent cannot access data from one goroutine in another goroutine. 

However, when a new goroutine is started on an existing goroutine, the `runtime.newproc1` method is called to create a new goroutine based on the existing one. 
The current solution used by the Agent is to, after the method execution is finished, use the `defer` command so that the Agent can access both the previous and the new goroutine. 
At this point, the data in the custom fields is copied. The purpose of copying is to prevent panic caused by the same object being accessed in multiple goroutines.

The specific operation process is as follows:

1. **Write the copy method**: Create a method for copying data from the custom fields.
2. **Insert code into newproc1**: Insert the `defer` code, intercept the `g` objects before and after the execution, and call the copy method to assign values to the custom fields' data.

## Agent with Dependency

Since SkyWalking Go Agent is based on compile-time enhancement, it cannot introduce third-party modules. 
For example, when SkyWalking Agent communicates with OAP, it needs to exchange data through the `gRPC` protocol. 
If the user does not introduce the gRPC module, it cannot be completed.

Due to this problem, users need to introduce relevant modules to complete the basic dependency functions. 
The main key modules that users currently need to introduce include:

1. **uuid**: Used to generate UUIDs, mainly for `TraceID` generation.
2. **errors**: To encapsulate error content.
3. **gRPC**: The basic library used for communication between SkyWalking Go Agent and the Server.
4. **skywalking-goapi**: The data protocol for communication between Agent and Server in SkyWalking.

### Agent Core Copy

To simplify the complexity of using Agent, the SkyWalking Go introduced by users only contains the user usage API and code import. 
The Agent Core code would be dynamically added during hybrid compilation, so when the Agent releases new features, 
users only need to upgrade the Agent enhancement program without modifying the references in the program.

### Code Import

You can see a lot of `imports.go` files anywhere in the SkyWalking Go, such as [imports.go in the root directory](../../../imports.go), but there is no actual code. 
This is because, during hybrid compilation, if the code to be compiled references other libraries, 
such as `os`, `fmt`, etc., they need to be referenced through the **importcfg** file during compilation.

The content of the `importcfg` file is shown below, which specifies the package dependency information required for all Go files to be compiled in the current package path.

```
packagefile errors=/var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build2774248373/b006/_pkg_.a
packagefile internal/itoa=/var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build2774248373/b027/_pkg_.a
packagefile internal/oserror=/var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build2774248373/b035/_pkg_.a
```

So when the file is copied and added to the compilation process, the relevant dependency libraries need to be declared in `importcfg`. 
Therefore, by predefining `import` in the project, the compiler can be forced to introduce the relevant libraries during compilation, 
thus completing the dynamic enhancement operation.

## Plugin with Agent Core

As mentioned in the previous section, it is not possible to dynamically add dependencies between modules. 
Agent can only modify the `importcfg` file to reference dependencies if we are sure that the previous dependencies have already been loaded, 
but this is often impractical. For example, Agent cannot introduce dependencies from the plugin code into the Agent Core, 
because the plugin is unaware of the Agent's existence. This raises a question: how can agent enable communication between plugins and Agent Core?

Currently, agent employ the following method: a global object is introduced in the `runtime` package, provided by Agent Core. 
When a plugin needs to interact with Agent Core, it simply searches for this global object from `runtime` package. The specific steps are as follows:

1. **Global object definition**: Add a global variable when the `runtime` package is loaded and provide corresponding set and get methods.
2. **Set the variable when the Agent loads**: When the Agent Core is copied and enhanced, import the method for setting the global variable and initialize the object in the global variable.
3. **Plugin enhancement**: When the plugin is enhanced, import the method for getting the global variable and the interface definition for the global variable. 
At this point, we can access the object set in Agent Core and use the defined interface for the plugin to access methods in Agent Core.

### Limitation

Since the communication between the plugin API and Agent Core is through an interface, and the plugin API is copied in each plugin, 
they can only transfer **basic data types or any(`interface{}`) type**. The reason is that when additional types are transferred, 
agent would be copied multiple times, so the types transferred in the plugin are not consistent with the types in Agent Core, 
as the types also need to be defined multiple times. 

Therefore, when communicating, they only pass structured data through **any type**, and when the Agent Core or plugin obtains the data, a type cast is simply required.

## Debugging

Based on the introductions in the previous sections, both Agent Core and plugin code are **dynamically copied/modified** into the target package. 
So, how can we debug the program during development to identify issues?

Our current approach consists of the following steps:

1. **Inform the source code location during flag**: Enhance the debug parameters during compilation and inform the system path, for example: `-toolexec "/path/to/agent -debug /path/to/code"`
2. **Get the original file path**: Find the absolute location of the source code of the file to be copied based on the rules.
3. **Introduce the `//line` directive**: Add the `//line` directive to the copied target file to inform the compiler of the location of the original file after copying.

At this point, when the program is executed, developer can find the original file to be copied in the source code.