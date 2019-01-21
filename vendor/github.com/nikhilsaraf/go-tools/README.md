# go-tools
A collection of tools for Golang

The [multithreading](multithreading) library currently supports a ThreadTracker struct that allows you to easily manage goroutines.
    - It allows you to create new goroutines and Wait for all of them to finish.
    - It allows you to set deferred functions inside the new goroutine it creates.
    - You can easily handle panics inside these goroutines by passing a panic handler as a deferred function.
