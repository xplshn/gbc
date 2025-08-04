main() {
    extrn printf;
    auto i;

    // Ascending: 0 to 15
    i = 0;
    while (i <= 15) {
        printf("%d\n", i);
        i++;
    }

    // Separator
    printf("----\n");

    // Descending: 15 to 0
    i = 15;
    while (i >= 0) {
        printf("%d\n", i);
        i--;
    }
}
