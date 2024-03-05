# Test 2: test resuse load balancer for many ingress by specific annotation `vks.vngcloud.vn/load-balancer-id`

When create resource ingress in k8s with annotation `kubernetes.io/ingress.class: "vngcloud"`, it'll create a new load-balancer with a unique name. But it can reuse by specific annotation `vks.vngcloud.vn/load-balancer-id`.

1. Create a new ingress resource and get load-balancer id

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn2
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 1111
    ```

2. Fill out in this ingress and apply. It'll update load-balancer that can work for both ingress. Test again with curl

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn22
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

3. Delete a ingress, it'll delete policy and rule relate to this ingress. Check this section carefully. Almost bug from here.

4. Clear test
