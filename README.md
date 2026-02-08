openssl genpkey -algorithm RSA -out private.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -pubout -in private.pem -out public.pem

TODO:
1. after adding observability check if configs are enough
2. make sure rate limiting is getting the correct IP
3. pass context to redis?
