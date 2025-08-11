G[10];
V[5];
B; C;

/* Function to demonstrate G[B](C) and G[B][C] */
demo() {
    G[B](C); /* Call function pointer at G[B] with C */
    V[1] = G[B][C]; /* Assign G[B][C] to V[1] */
    printf("%s\n", V[1] + '0'); /* Output result */
}

main() {
    B = 2; /* Set index */
    C = 2; /* Set value */
    G[2] = printf; /* Store function pointer */
    G[3] = V; /* Store vector pointer */
    V[2] = 2; /* Set V[2] for G[3][2] */
    G[B] = C; /* Assign C to G[2] */
    demo(); /* Call demo function */
}
