openssl genpkey -algorithm RSA -out private.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -pubout -in private.pem -out public.pem

TODO:
1. after adding observability check if configs are enough
2. make sure rate limiting is getting the correct IP - done
3. pass context to redis? - done
4. OID needs to be present in JWT to identify user, if not the rate limiting will be applied to IP and break fast
5. develop airflow endpoint to invalidate cached tables - done
