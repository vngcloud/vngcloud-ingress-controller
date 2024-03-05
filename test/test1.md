# Test 1: test create http and https listener

1. Apply ingress in file [test1-ingress.yaml](./test1-ingress.yaml)

    ```bash
    kubectl apply -f test1-ingress.yaml
    ```

2. Wait for lb active

3. Test with `{{IP}}` from vLB

    ```bash
    curl https://{{IP}}/2 -k
    # {\"received_path\":\"Port: 2222, path: /2\"}
    curl https://{{IP}}/22 -k
    # "{\"received_path\":\"Port: 1111, path: /22\"}"
    curl https://{{IP}}/webb -k
    # "{\"received_path\":\"Port: 1111, path: /webb\"}"

    curl -H "Host: example.com" https://{{IP}}/webb -k
    # "{\"received_path\":\"Port: 1111, path: /webb\"}"
    curl -H "Host: example2.com" https://{{IP}}/webb -k
    # "{\"received_path\":\"Port: 1111, path: /webb\"}"
    curl -H "Host: example.com" https://{{IP}}/3 -k
    # "{\"received_path\":\"Port: 3333, path: /3\"}", resp)"

    curl -H "Host: kkk.example.com" https://{{IP}}/4 -k
    # "{\"received_path\":\"Port: 4444, path: /4\"}"
    ```

4. Clear the test

    ```bash
    kubectl delete -f test1-ingress.yaml
    ```
