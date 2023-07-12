# Support Transport Layer Security (TLS)
Transport Layer Security (TLS) is a very common security way when transport data through Internet.
In some use cases, end users report the background:

> Target(under monitoring) applications are in a region, which also named VPC,
at the same time, the SkyWalking backend is in another region (VPC).
>
> Because of that, security requirement is very obvious.

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
- Enable (m)TLS on the OAP server side, [read more on this documentation](https://skywalking.apache.org/docs/main/v9.5.0/en/setup/backend/grpc-security/).
- Following the configuration to enable (m)TLS on the agent side.

| Name                                            | Environment Variable                              | Required Type | Description                                                         |
|-------------------------------------------------|---------------------------------------------------|---------------|---------------------------------------------------------------------|
| reporter.grpc.tls.enable                        | SW_AGENT_REPORTER_GRPC_TLS_ENABLE                 | TLS/mTLS      | Enable (m)TLS on the gRPC reporter.                                 |
| reporter.grpc.tls.ca_path                       | SW_AGENT_REPORTER_GRPC_TLS_CA_PATH                | TLS           | The path of the CA certificate file. eg: `/path/to/ca.cert`.        |
| reporter.grpc.tls.client.key_path               | SW_AGENT_REPORTER_GRPC_TLS_CLIENT_KEY_PATH        | mTLS          | The path of the client private key file, eg: `/path/to/client.pem`. |
| reporter.grpc.tls.client.client_cert_chain_path | SW_AGENT_REPORTER_GRPC_TLS_CLIENT_CERT_CHAIN_PATH | mTLS          | The path of the client certificate file, eg: `/path/to/client.crt`. |
| reporter.grpc.tls.insecure_skip_verify          | SW_AGENT_REPORTER_GRPC_TLS_INSECURE_SKIP_VERIFY   | TLS/mTLS      | Skip the server certificate and domain name verification.           |