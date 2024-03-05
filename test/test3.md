# Test 3: test default backend

When creating an ingress resource, we can define a rule that routes traffic to a default group and group to route all traffic that does not match the rule by define `defaultBackend:`.

1. Create ingress resource with `defaultBackend:`

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn3
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 1111
    ```

2. Test with curl

3. Create a new ingress resource with annotation to vLB above.

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn33
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
        vks.vngcloud.vn/load-balancer-id: "____________ID____________"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 2222
    ```

4. Test with curl

5. Clear test
