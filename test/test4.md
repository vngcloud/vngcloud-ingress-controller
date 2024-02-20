# Test 4: test update https cert in secret

The user can pass the certificate to the https listener by passing the secret in `spec.tls`. It will generate a TLS/SSL certificate in the VNGCLOUD portal and the load balancer will use it. When the certificate expires or changes, the user must update the secret, a new certificate will be created, the load balancer must update.
