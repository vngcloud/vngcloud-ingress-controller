# Test 7: test many cluster use a load balancer

A load-balancer can reuse in many ingress in a cluster if in a same subnet. Similarly, many cluster can reuse a load-balancer.

1. Apply pre-test to 2 clusters A, B

2. Apply this ingress to A

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn8
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 2222
    ```

3. Apply this ingress to B, with ID from vLB create above

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn88
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
        vks.vngcloud.vn/load-balancer-id: "____________ID____________"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 3333
    ```

4. Check if 2 ingress work ok, use a same vLB

5. Delete ingress in cluster B, check if vLB update succesfully.
