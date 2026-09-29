# Support Transport Layer Security (TLS)
Transport Layer Security (TLS) is a very common security way when transport data through Internet.
In some use cases, end users report the background:

## Creating SSL/TLS Certificates

The first step is to generate certificates and key files for encrypting communication. This is
fairly straightforward: use `openssl` from the command line.

Use this [script](../../../tools/TLS/tls_key_generate.sh) if you are not familiar with how to generate key files.

We need the following files:
- `client.pem`: A private RSA key to sign and authenticate the public key. It's either a PKCS#8(PEM) or PKCS#1(DER).
- `client.crt`: Self-signed X.509 public keys for distribution.
- `ca.crt`: A certificate authority public key for a client to validate the server's certificate.

## Authentication Mode
- Find `ca.crt`, and use it at client side. In `mTLS` mode, `client.crt` and `client.pem` are required at client side.
- Find `server.crt`, `server.pem` and `ca.crt`. Use them at server side. Please refer to `gRPC Security` of the OAP server doc for more details.

## Enable TLS
- Enable (m)TLS on the OAP server side, [read more on this documentation](https://skywalking.apache.org/docs/main/next/en/setup/backend/grpc-security/).
- Following the configuration to enable (m)TLS on the agent side.

| Name                                            | Environment Variable                              | Required Type | Description                                                         |
|-------------------------------------------------|---------------------------------------------------|---------------|---------------------------------------------------------------------|
| reporter.grpc.tls.enable                        | SW_AGENT_REPORTER_GRPC_TLS_ENABLE                 | TLS/mTLS      | Enable (m)TLS on the gRPC reporter.                                 |
| reporter.grpc.tls.ca_path                       | SW_AGENT_REPORTER_GRPC_TLS_CA_PATH                | TLS           | The path of the CA certificate file. eg: `/path/to/ca.cert`.        |
| reporter.grpc.tls.client.key_path               | SW_AGENT_REPORTER_GRPC_TLS_CLIENT_KEY_PATH        | mTLS          | The path of the client private key file, eg: `/path/to/client.pem`. |
| reporter.grpc.tls.client.client_cert_chain_path | SW_AGENT_REPORTER_GRPC_TLS_CLIENT_CERT_CHAIN_PATH | mTLS          | The path of the client certificate file, eg: `/path/to/client.crt`. |
| reporter.grpc.tls.insecure_skip_verify          | SW_AGENT_REPORTER_GRPC_TLS_INSECURE_SKIP_VERIFY   | TLS/mTLS      | Skip the server certificate and domain name verification.           |

## Multi-address backends

Configure comma-separated backends:

```yaml
reporter:
  grpc:
    backend_service: oap-a:11800,oap-b:11800
```

Use `host:port` entries with numeric ports from 1 to 65535 and brackets around
IPv6 addresses (`[::1]:11800`). The agent trims whitespace, ignores empty entries,
and removes duplicates. Invalid entries in a comma-separated list are skipped
with a warning during initialization. These warnings are not repeated when
reconnecting. A list that normalizes to one address uses the existing
single-address connection behavior. If no valid entries remain, the gRPC
reporter is disabled (discard reporter) and the application continues; the Kafka
reporter still initializes and runs without CDS when the gRPC backend list is
invalid.

A single non-`host:port` token such as `dns:///oap:11800` or `unix:///tmp/oap.sock`
is passed through unchanged so legacy gRPC target URIs keep working.

For multiple addresses, the agent creates one gRPC channel and shuffles its
static endpoint list once. Native `pick_first` selects the first reachable
backend in that order and keeps using it until its transport fails. Resolver
refreshes preserve the shuffled order. The configured list stays fixed; Go's TCP
dialer resolves hostname entries when opening a connection, with no periodic DNS
refresh of the list. Each address has its own dial/handshake timeout covering
TCP, TLS, and HTTP/2 negotiation so a silent peer cannot starve the rest of the
connect budget. Client keepalive probes help
detect half-open peers so `pick_first` can move to a standby; keepalive Time
is at least 30s and otherwise follows `reporter.check_interval` plus 10s so
healthy management heartbeats suppress pings against OAP's default server
policy.

Telemetry uses long-lived Collect streams. A failed send is discarded and
**never replayed**: the backend may already have accepted it. Automatic gRPC
retries apply only to the idempotent `reportInstanceProperties` RPC on
`UNAVAILABLE`, with at most three attempts.

RPC deadlines and stream cancellation bound individual operations. Client
keepalive closes the transport to a half-open peer so `pick_first` can move to
a standby, and a bounded send timeout unblocks a stuck Send. Established
streams are not canceled based on channel state, so streams draining after a
graceful GOAWAY can finish.

### TLS ServerName per address

Each static resolver endpoint sets `ServerName` from that address's host so TLS
SNI and certificate verification follow the dialed backend. An explicit
server-name override in transport credentials still takes precedence. Prefer
hostnames that match each backend's certificate when enabling TLS across a
multi-address list.

### Authentication failures

`UNAUTHENTICATED` / `PERMISSION_DENIED` are not retryable in the multi-backend
service configuration. A bad shared credential will not usefully rotate across
backends. The connection interceptor emits an authentication diagnostic at most
once per 30 seconds and keeps the existing channel. Check
`reporter.grpc.authentication` and the OAP authentication configuration.
