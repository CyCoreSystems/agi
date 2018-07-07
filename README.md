# Asterisk AGI library for Go (golang)

[![Build Status](https://travis-ci.org/CyCoreSystems/agi.png)](https://travis-ci.org/CyCoreSystems/agi) [![](https://godoc.org/github.com/CyCoreSystems/agi?status.svg)](http://godoc.org/github.com/CyCoreSystems/agi)

This is an Asterisk AGI interface library which may be used for both classical
AGI, with a standalone executable, or FastAGI, with a TCP server.

```go
package main

import "github.com/CyCoreSystems/agi"

func main() {
   a := agi.NewStdio()

   a.Answer()
   err := a.Set("MYVAR", "foo")
   if err != nil {
      panic("failed to set variable MYVAR")
   }
   a.Hangup()
}
```

## Standalone AGI executable

Use `agi.NewStdio()` to get an AGI reference when running a standalone
executable.

For a TCP server, register a HandlerFunc to a TCP port:

```go
package main

import "github.com/CyCoreSystems/agi"

func main() {
   agi.Listen(":8080", handler)
}

func handler(a *agi.AGI) {
   defer a.Close()

   a.Answer()
   err := a.Set("MYVAR", "foo")
   if err != nil {
      panic("failed to set variable MYVAR")
   }
   a.Hangup()
}
```

