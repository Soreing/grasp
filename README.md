# Go Resource Auto Scaling Pool
Grasp is a resource pool which manages the creation and deletion of resources as needed after sudden increase and decrease in request for resources.

## Usage
The resource pool can only work with objects that implement the Poolable interface. The interface defines how to release the object when it's been idle for too long.
```golang
type Item struct { 
    // Some fields
}
func (i *Item) PoolRelease() {
	fmt.Println("released")
}
```

To create a resource pool, you need to provide a TTL value for the maximum lifetime of idle resources, an upper limit for the number of resources in the pool, as well as a constructor that creates new Poolable resources.
```golang
pl := NewPool(5, time.Second, func() Poolable {
	return &Item{/* fields or a constuctor*/}
})
```

To get a resource from the pool you need to request for one. The `Acquire` function returns a resource, a done function, and an error. If no errors occured,the the `done` function must be called to release the resource when it is no longer needed.

If there are idle resources, the function returns one. If there are no idle resources, one will be created up to the limit. If there are no resources available, the call blocks until a resource is freed up or the context is canceled.
```golang
res, done, err := pl.Acquire(context.TODO())
```

To stop using the pool, call the `Close` function, which cleans up all the resources and waits for all in use resources to be released before returning.
```golang
pl.Close()
```