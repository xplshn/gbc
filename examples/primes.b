main() {
    extrn printf;
    auto n, i, j, isprime, count;

    count = 0;
    n = 2;

    while (n < 10000) {
        isprime = 1;
        i = 2;

        while (i * i <= n) {
            if (n % i == 0)
                isprime = 0;
            i = i + 1;

            if (isprime == 0)
                i = n; /* force exit: i*i > n */
        }

        if (isprime)
            count = count + 1;

        n = n + 1;
    }

    printf("Found %d primes\n", count);
}
