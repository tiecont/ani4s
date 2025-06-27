package lib

import (
	"github.com/gorilla/websocket"
	"sync"
)

var (
	UserSocketMap = make(map[uint]*websocket.Conn)
	MapMutex      = sync.RWMutex{} // processing concurrent map access
)

func SetUserSocket(userID uint, conn *websocket.Conn) {
	MapMutex.Lock()
	defer MapMutex.Unlock()
	UserSocketMap[userID] = conn
}

func GetUserSocket(userID uint) (*websocket.Conn, bool) {
	MapMutex.RLock()
	defer MapMutex.RUnlock()
	conn, exists := UserSocketMap[userID]
	return conn, exists
}

func RemoveUserSocket(userID uint) {
	MapMutex.Lock()
	defer MapMutex.Unlock()
	delete(UserSocketMap, userID)
}
