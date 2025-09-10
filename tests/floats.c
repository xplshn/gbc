#include <stdio.h>

int main(void) {
    char buffer[100];
    int x = 2;
    double y = 3.14;
    double z = 2.71;

    // --- Integer Test ---
    printf("--- Testing Integer Switch ---\n");
    switch (x) {
        case 1:
            printf("x is one\n");
            break;
        case 2:
        case 3:
            printf("x is two or three\n");
            break;
        default:
            printf("x is something else\n");
            break;
    }
    printf("Integer test passed.\n\n");

    // --- Float Test ---
    printf("--- Testing Float Operations ---\n");
    double result = y * z; // 3.14 * 2.71 = 8.5094

    if (result > 8.5 && result < 8.51) {
        sprintf(buffer, "Float multiplication successful: %f * %f = %f\n", y, z, result);
        printf("%s", buffer);
    } else {
        sprintf(buffer, "Float multiplication failed: %f * %f = %f\n", y, z, result);
        printf("%s", buffer);
    }

    double div_res = y / 2.0; // 3.14 / 2.0 = 1.57
    if (div_res > 1.56 && div_res < 1.58) {
        sprintf(buffer, "Float division successful: %f / 2.0 = %f\n", y, div_res);
        printf("%s", buffer);
    } else {
        sprintf(buffer, "Float division failed: %f / 2.0 = %f\n", y, div_res);
        printf("%s", buffer);
    }

    return 0;
}
