# socks5 over multiple network zones tunneled through a tls tcp connection

```ascii
+-------------------+   +--------------+   +--------------+   +-----------------+
| source zone       |   |  zone A      |   | zone ...     |   | target zone     |
| +---------------+ |   |              |   |              |   | +-------------+ |
| |               | |   |              |   |              |   | |             | |
| | client A      | |   |              |   |              |   | | service A   | |
| |               | |   |              |   |              |   | |             | |
| +-------+-------+ |   |              |   |              |   | +------^------+ |
|         |         |   |              |   |              |   |        |        |
| +-------v-------+ |   | +----------+ |   | +----------+ |   | +------+------+ |
| |               | |   | |          | |   | |          | |   | |             | |
| | tlssocksproxy +-------> tcpproxy +-------> tcpproxy +-------> tlssocks    | |
| |               | |   | |          | |   | |          | |   | |             | |
| +-------^-------+ |   | +----------+ |   | +----------+ |   | +------+------+ |
|         |         |   |              |   |              |   |        |        |
| +-------+-------+ |   |              |   |              |   | +------v------+ |
| |               | |   |              |   |              |   | |             | |
| | client ...    | |   |              |   |              |   | | service ... | |
| |               | |   |              |   |              |   | |             | |
| +---------------+ |   |              |   |              |   | +-------------+ |
|                   |   +--------------+   +--------------+   |                 |
|                   |                                         |                 |
|                   +----------------------------------------->                 |
|                   |       tls connection over n zones       |                 |
+-------------------+                                         +-----------------+
```
## example project

Example docker-compose project in docker/localtest - should help to understand the configuration

```bash
# run example project
make docker-local-test
```

```bash
# try it

# works for jan
curl -vvv --proxy socks5h://jan:secret@127.0.0.1:8000 http://echo:8080/
*   Trying 127.0.0.1...
* TCP_NODELAY set
* SOCKS5 communication to echo:8080
* SOCKS5 request granted.
* Connected to 127.0.0.1 (127.0.0.1) port 8000 (#0)
> GET / HTTP/1.1
> Host: echo:8080
> User-Agent: curl/7.54.0
> Accept: */*
>
< HTTP/1.1 200 OK
< X-App-Name: http-echo
< X-App-Version: 0.2.3
< Date: Mon, 06 Aug 2018 09:44:03 GMT
< Content-Length: 12
< Content-Type: text/plain; charset=utf-8
<
hello-world
* Connection #0 to host 127.0.0.1 left intact

# does not work for peter
curl -vvv --proxy socks5h://peter:secret@127.0.0.1:8000 http://echo:8080/
*   Trying 127.0.0.1...
* TCP_NODELAY set
* SOCKS5 communication to echo:8080
* Can't complete SOCKS5 connection to 0.0.0.0:0. (2)
* Closing connection 0
curl: (7) Can't complete SOCKS5 connection to 0.0.0.0:0. (2)

```

## tlssocksproxy

Opens an tls encrypted connection to tlssocks - optionally through one or multiple tcpproxies and makes it available as a "normal" socks5 server.

```bash
# running a tlssocksproxy locally to connect a remote tlssocks using tls encryption
docker run --rm -p="8000:8000" foomo/tlssocksproxy:latest -addr="0.0.0.0:8000" -server="tlssocks.example.com:8765"
```

## tcpproxy

Very light weight wrapper around googles [https://github.com/google/tcpproxy](https://github.com/google/tcpproxy) - can be daisychained.

## tlssocks

Based on [github.com/armon/go-socks5](github.com/armon/go-socks5) wrapped by [https://golang.org/pkg/crypto/tls/](https://golang.org/pkg/crypto/tls/).

- tls protection
- authentication with bcrypt hashed passwords (htpasswd compatible)
- destinations configuration

Managing credentials:

Can be done with good old htpasswd - in case of doubt `man htpasswd`

```bash
# set the password for a user in an existing file using bcrypt
htpasswd -B path/to/users.htpasswd <user-name>
```

Configuring destinations:

By default every proxy request is denied, except it is explicitly allowed through the destinations configuration. 

Destinations are configured on a name basis and allowed for ports and users.

WARNING - if you are running multiple vhosts on one destination all of them are accessible, since tlssocks is not inspecting the incoming traffic.

```yaml
---
# destination www.heise.de will be accessible trough ports 80, 443 for users jan and peter
www.heise.de:
  ports: 
    - 80
    - 443
  users:
    - jan
    - peter
# destination echo is acce
echo:
  ports: 
    - 8080
  users:
    - jan

www.google.com:
  ports: 
    - 443
  users:
    - peter
...
```

## caveats / todos

- only name based destinations are supported

## security concerns

All vhosts on a destination can be accessed through the allowed ports - there is no traffic inspection to prevent this.
