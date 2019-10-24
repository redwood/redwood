
# redwood

```sh
$ go run cmd/main.go
```



## HTTP Requests


- **Regular GET**
    ```
    GET <path>
    ```
    Returns the contents of the state tree at the given path.


- **Subscription GET**
    ```
    GET <path>
    Subscribe: <host>
    ```
    Creates an SSE keep-alive connection that pushes transactions to the subscriber.


- **Verify-Credentials GET**
    ```
    GET /
    Verify-Credentials: <challenge message>
    ```
    Requests that the receiving node sign the challenge message to prove which address represents their identity.





