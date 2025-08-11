main() {
    extrn printf;
    auto W, H, i, j, word_size;

    word_size = &0[1];
    W = 5;
    H = 4;

    auto matrix_data 20; // H * W = 4 * 5 = 20 words // :3

    // Allocate an array of pointers for the rows
    auto matrix 4;

    // Set row pointers to point into the data block
    i = 0;
    while (i < H) {
        matrix[i] = &matrix_data[i * W];
        i++;
    }

    i = 0;
    while (i < H) {
        j = 0;
        while (j < W) {
            matrix[i][j] = i * 10 + j;
            j++;
        }
        i++;
    }

    printf("Printing a %d by %d matrix\n", H, W);
    i = 0;
    while (i < H) {
        j = 0;
        while (j < W) {
            printf("%d\t", matrix[i][j]);
            j++;
        }
        printf("\n");
        i++;
    }
}
