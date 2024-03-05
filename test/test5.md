# Test 5: test scale minion

When scale minion up or down, pool member must update

1. Create a ingress

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
    name: linhpn5
    annotations:
        kubernetes.io/ingress.class: "vngcloud"
    spec:
    defaultBackend:
        service:
        name: goapp-debug
        port:
            number: 2222
    ```

2. Go to portal, change number of minion in cluster

3. Check if number of pool members is update
