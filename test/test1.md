# Test 1: test create http and https listener

1. Apply ingress in file [test1-ingress.yaml](./test1-ingress.yaml)

    ```bash
    kubectl apply -f test1-ingress.yaml
    ```

2. Wait for lb active

3. Test http listener

    ```bash
    curl -H "Host: https-example.foo.com" https://{{IP}}/webserver -k
    ```

4. Test https listener

    ```bash
    curl -H "Host: https-example.foo.com" https://{{IP}}/webserver -k
    ```

5. Clear the test

    ```bash
    kubectl delete -f test1-ingress.yaml
    ```
