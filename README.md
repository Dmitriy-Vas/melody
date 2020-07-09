# melody

[![Build Status](https://travis-ci.org/Dmitriy-Vas/melody.svg)](https://travis-ci.org/Dmitriy-Vas/melody)
[![Coverage Status](https://img.shields.io/coveralls/olahol/melody.svg?style=flat)](https://coveralls.io/r/olahol/melody)
[![GoDoc](https://godoc.org/github.com/Dmitriy-Vas/melody?status.svg)](https://godoc.org/github.com/Dmitriy-Vas/melody)

> :notes: Minimalist websocket framework for Go.

Melody is websocket framework based on [github.com/gorilla/websocket](https://github.com/gorilla/websocket)
that abstracts away the tedious parts of handling websockets. It gets out of
your way so you can write real-time apps. Features include:

* [x] Clear and easy interface similar to `net/http` or Gin.
* [x] A simple way to broadcast to all or selected connected sessions.
* [x] Message buffers making concurrent writing safe.
* [x] Automatic handling of ping/pong and session timeouts.
* [x] Store data on sessions.

## Install

```bash
go get -u github.com/Dmitriy-Vas/melody/v1
```

## [Example: chat](https://github.com/Dmitriy-Vas/melody/tree/master/examples/chat)

[![Chat](https://raw.githubusercontent.com/Dmitriy-Vas/melody/master/examples/chat/demo.gif "Demo")](https://github.com/Dmitriy-Vas/melody/tree/master/examples/chat)

Using [Gin](https://github.com/gin-gonic/gin):
```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/Dmitriy-Vas/melody"
	"net/http"
)

func main() {
	r := gin.Default()
	m := melody.New()

	r.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "index.html")
	})

	r.GET("/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		m.Broadcast(msg)
	})

	r.Run(":5000")
}
```

Using [Echo](https://github.com/labstack/echo):
```go
package main

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/Dmitriy-Vas/melody"
	"net/http"
)

func main() {
	e := echo.New()
	m := melody.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		http.ServeFile(c.Response().Writer, c.Request(), "index.html")
		return nil
	})

	e.GET("/ws", func(c echo.Context) error {
		m.HandleRequest(c.Response().Writer, c.Request())
		return nil
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		m.Broadcast(msg)
	})

	e.Logger.Fatal(e.Start(":5000"))
}
```

## [Example: gophers](https://github.com/Dmitriy-Vas/melody/tree/master/examples/gophers)

[![Gophers](https://raw.githubusercontent.com/Dmitriy-Vas/melody/master/examples/gophers/demo.gif "Demo")](https://github.com/Dmitriy-Vas/melody/tree/master/examples/gophers)

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/Dmitriy-Vas/melody"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type GopherInfo struct {
	ID, X, Y string
}

func main() {
	router := gin.Default()
	mrouter := melody.New()
	gophers := make(map[*melody.Session]*GopherInfo)
	lock := new(sync.Mutex)
	counter := 0

	router.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "index.html")
	})

	router.GET("/ws", func(c *gin.Context) {
		mrouter.HandleRequest(c.Writer, c.Request)
	})

	mrouter.HandleConnect(func(s *melody.Session) {
		lock.Lock()
		for _, info := range gophers {
			s.Write([]byte("set " + info.ID + " " + info.X + " " + info.Y))
		}
		gophers[s] = &GopherInfo{strconv.Itoa(counter), "0", "0"}
		s.Write([]byte("iam " + gophers[s].ID))
		counter += 1
		lock.Unlock()
	})

	mrouter.HandleDisconnect(func(s *melody.Session) {
		lock.Lock()
		mrouter.BroadcastOthers([]byte("dis "+gophers[s].ID), s)
		delete(gophers, s)
		lock.Unlock()
	})

	mrouter.HandleMessage(func(s *melody.Session, msg []byte) {
		p := strings.Split(string(msg), " ")
		lock.Lock()
		info := gophers[s]
		if len(p) == 2 {
			info.X = p[0]
			info.Y = p[1]
			mrouter.BroadcastOthers([]byte("set "+info.ID+" "+info.X+" "+info.Y), s)
		}
		lock.Unlock()
	})

	router.Run(":5000")
}
```

### [More examples](https://github.com/Dmitriy-Vas/melody/tree/master/examples)

## [Documentation](https://godoc.org/github.com/Dmitriy-Vas/melody)

## Contributors

* Ola Holmström (@olahol)
* Shogo Iwano (@shiwano)
* Matt Caldwell (@mattcaldwell)
* Heikki Uljas (@huljas)
* Robbie Trencheny (@robbiet480)
* yangjinecho (@yangjinecho)

## FAQ

If you are getting a `403` when trying  to connect to your websocket you can [change allow all origin hosts](http://godoc.org/github.com/gorilla/websocket#hdr-Origin_Considerations):

```go
m := melody.New()
m.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
```
