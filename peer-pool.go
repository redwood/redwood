package redwood

//import (
//    "context"
//    "fmt"
//    "sync"
//    "time"
//
//    "github.com/pkg/errors"
//)
//
//type peerPool struct {
//    keepalive bool
//    transport Transport
//
//    chPeers       chan *peerWrapper
//    chNeedNewPeer chan struct{}
//    chProviders   <-chan Peer
//    ctx           context.Context
//    cancel        func()
//    foundPeers    map[string]bool
//}
//
//func newPeerPool(ctx context.Context, transport Transport, refHash Hash, concurrentConns uint, keepalive bool) (*peerPool, error) {
//    ctxInner, cancel := context.WithCancel(ctx)
//    foundPeers := make(map[string]bool)
//
//    chProviders, err := transport.ForEachProviderOfRef(ctxInner, refHash)
//    if err != nil {
//        return nil, err
//    }
//
//    p := &peerPool{
//        keepalive:     keepalive,
//        transport:     transport,
//        chPeers:       make(chan *peerWrapper, concurrentConns),
//        chNeedNewPeer: make(chan struct{}, concurrentConns),
//        chProviders:   chProviders,
//        ctx:           ctxInner,
//        cancel:        cancel,
//    }
//
//    // When a message is sent on the `needNewPeer` channel, this goroutine attempts to take a peer
//    // from the `chProviders` channel, open a peerWrapper to it, and add it to the pool.
//    go func() {
//        // defer close(p.chPeers)
//
//        for {
//            select {
//            case <-p.chNeedNewPeer:
//            case <-p.ctx.Done():
//                return
//            }
//
//            var conn *peerWrapper
//        FindPeerLoop:
//            for {
//                var peerID string
//                select {
//                case peer, open := <-p.chProviders:
//                    if !open {
//                        p.chProviders, err = transport.ForEachProviderOfRef(p.ctx, refHash)
//                        if err != nil {
//                            fmt.Println("[peer pool] error requesting blob providers:", err) // @@TODO: logger
//                            return
//                        }
//                        continue FindPeerLoop
//                    } else if foundPeers[peer.ID()] {
//                        continue FindPeerLoop
//                    }
//                    peerID = peer.ID()
//                    foundPeers[peerID] = true
//
//                case <-p.ctx.Done():
//                    return
//                }
//
//                fmt.Printf("[peer pool] found peer %v\n", peerID) // @@TODO: logger
//                conn = newPeerConn(ctxInner, p.transport, peerID)
//
//                break
//            }
//
//            select {
//            case p.chPeers <- conn:
//            case <-p.ctx.Done():
//                return
//            }
//        }
//    }()
//
//    // This goroutine fills the peer pool with the desired number of peers.
//    go func() {
//        for i := uint(0); i < concurrentConns; i++ {
//            select {
//            case <-p.ctx.Done():
//                return
//            case p.chNeedNewPeer <- struct{}{}:
//            }
//        }
//    }()
//
//    return p, nil
//}
//
//func (p *peerPool) Close() error {
//    // p.cancel()
//
//    p.chNeedNewPeer = nil
//    p.chProviders = nil
//
//    return nil
//}
//
//func (p *peerPool) GetConn() (*peerWrapper, error) {
//    select {
//    case conn, open := <-p.chPeers:
//        if !open {
//            return nil, errors.New("connection closed")
//        }
//
//        err := conn.EnsureConnected(conn.ctx)
//        return conn, errors.WithStack(err)
//
//    case <-p.ctx.Done():
//        return nil, errors.WithStack(p.ctx.Err())
//    }
//}
//
//func (p *peerPool) ReturnConn(conn *peerWrapper, strike bool) {
//    if strike {
//        // Close the faulty connection
//        conn.CloseConn()
//
//        // Try to obtain a new peer
//        select {
//        case p.chNeedNewPeer <- struct{}{}:
//        case <-p.ctx.Done():
//        }
//
//    } else {
//        if !p.keepalive {
//            conn.CloseConn()
//        }
//
//        // Return the peer to the pool
//        select {
//        case p.chPeers <- conn:
//        case <-p.ctx.Done():
//        }
//    }
//}
//
//type peerWrapper struct {
//    Peer
//    transport     Transport
//    peerID        string
//    ctx           context.Context
//    mu            *sync.Mutex
//    closed        bool
//    closedForever bool
//}
//
//func newPeerConn(ctx context.Context, transport Transport, peerID string) *peerWrapper {
//    conn := &peerWrapper{
//        transport:     transport,
//        peerID:        peerID,
//        Peer:          nil,
//        ctx:           ctx,
//        mu:            &sync.Mutex{},
//        closed:        true,
//        closedForever: false,
//    }
//
//    go func() {
//        <-ctx.Done()
//        conn.closeForever()
//    }()
//
//    return conn
//}
//
//func (conn *peerWrapper) EnsureConnected(ctx context.Context) error {
//    conn.mu.Lock()
//    defer conn.mu.Unlock()
//
//    if conn.closedForever {
//        fmt.Println("[peer conn] peerWrapper is closedForever, refusing to reopen")
//        return nil
//    }
//
//    if conn.Peer == nil {
//        fmt.Println("[peer conn] peerWrapper.Peer is nil, opening new connection")
//
//        // @@TODO: make context timeout configurable
//        ctxConnect, cancel := context.WithTimeout(conn.ctx, 15*time.Second)
//        defer cancel()
//
//        peer, err := conn.transport.GetPeer(ctxConnect, conn.peerID)
//        if err != nil {
//            return errors.WithStack(err)
//        }
//
//        fmt.Println("[peer conn] peerWrapper.Peer successfully opened")
//        conn.Peer = peer
//    }
//
//    return nil
//}
//
//func (conn *peerWrapper) close() error {
//    if !conn.closed {
//        fmt.Printf("[peer conn] closing peerWrapper %v\n", conn.peerID)
//
//        err := conn.Peer.CloseConn()
//        if err != nil {
//            fmt.Println("[peer conn] error closing peerWrapper:", err)
//        }
//        conn.closed = true
//
//    } else {
//        fmt.Printf("[peer conn] already closed: peerWrapper %v\n", conn.peerID)
//    }
//    return nil
//}
//
//func (conn *peerWrapper) closeForever() error {
//    conn.mu.Lock()
//    defer conn.mu.Unlock()
//
//    conn.closedForever = true
//    return conn.close()
//}
//
//func (conn *peerWrapper) CloseConn() error {
//    conn.mu.Lock()
//    defer conn.mu.Unlock()
//
//    return conn.close()
//}
