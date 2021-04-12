
# HTTP Requests

- [x] **Regular GET**
    ```
    GET /  
    [Version: deadbeef]
    ```

    Returns a single response containing a state.



- [ ] **Span GET**
    ```
    GET /
    Parents: abc, def
    [Version: deadbeef]
    ```

    Returns a set of versions connecting the version and its parents.  If `Version` is absent, it is assumed to be whichever version the recipient considers most recent.


- [x] **Subscribe and fetch history**
    ```
    GET /
    [Parents: <abc, def | genesis tx id>]
    Subscribe: keep-alive
    ```

    Returns a set of versions connecting the version to current HEAD, and then subscribe to future updates.  Over a regular HTTP transport, the recipient must issue a `peerid` cookie for identifying the subscriber.  If `Parents` are missing, the subscription starts from the current HEAD.  If `Parents` is `genesis`, the entire history is fetched.


- [ ] **FORGET subscription**
    ```
    FORGET /
    [Cookie: peerid=deadbeef]
    ```

    Ends a subscription.  Over a regular HTTP transport, it's necessary to include a server-assigned `peerid` cookie to identify the requester.


------------

- [ ] **Traditional PUT/POST/PATCH**
    ```
    PUT/POST/PATCH
    Signature: deadbeef
    Version: randomidblabla

    { "messages": [ { "text": "hi" } ] }
    ```

    Regular HTTP-style state update.  Requires the receiver to either clobber the existing state or figure out how to merge it.


- [x] **Canonical Braid PUT/POST/PATCH**
    ```
    PUT /
    Signature: deadbeef
    Patch-Type: braid
    [Version: randomidblabla]
    [Parents: abc, def]

    .shrugisland.talk0.messages[1:1] = [{"text":"hi"}]
    .shrugisland.talk0.messages[2:2] = [{"text":"have a meme"}]
    ```

    Regular patch.

    - [ ] If `Version` is missing, the recipient assigns it.  (**NOTE**: this only makes sense in a star topology with a traditional server.  Should we consider this invalid in other cases, and if so, how do we detect it?  We might need a stronger concept of an "authoritative" peer, i.e., an owner of the state tree identified by a given domain/hostname.)
    - [ ] If `Parents` are missing, the recipient assumes that the parents are whichever leaves it currently knows about.




