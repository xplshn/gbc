#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>

typedef struct Point {
    int x;
    int y;
    unsigned char *name;
} Point;

typedef enum Color {
    RED,
    GREEN,
    BLUE,
    YELLOW
} Color;

void demo_integer_arrays(void) {
    printf("\n-- Integer arrays --\n");

    // Signed integer types
    int     int_array[3];
    int8_t  int8_array[3];
    int16_t int16_array[3];
    int32_t int32_array[3];
    int64_t int64_array[3];

    // Unsigned integer types
    unsigned int uint_array[3];
    uint8_t  uint8_array[3];
    uint16_t uint16_array[3];
    uint32_t uint32_array[3];
    uint64_t uint64_array[3];

    // Byte type
    uint8_t byte_array[3];

    int_array[0] = -100;
    int_array[1] = 0;
    int_array[2] = 100;
    printf("int array: [%d, %d, %d]\n", int_array[0], int_array[1], int_array[2]);

    int8_array[0] = -50;
    int8_array[1] = 0;
    int8_array[2] = 50;
    printf("int8 array: [%d, %d, %d]\n", int8_array[0], int8_array[1], int8_array[2]);

    int16_array[0] = -1000;
    int16_array[1] = 0;
    int16_array[2] = 1000;
    printf("int16 array: [%d, %d, %d]\n", int16_array[0], int16_array[1], int16_array[2]);

    int32_array[0] = -100000;
    int32_array[1] = 0;
    int32_array[2] = 100000;
    printf("int32 array: [%d, %d, %d]\n", int32_array[0], int32_array[1], int32_array[2]);

    int64_array[0] = -1000000;
    int64_array[1] = 0;
    int64_array[2] = 1000000;
    printf("int64 array: [%lld, %lld, %lld]\n",
           (long long)int64_array[0], (long long)int64_array[1], (long long)int64_array[2]);

    uint_array[0] = 10;
    uint_array[1] = 20;
    uint_array[2] = 30;
    printf("uint array: [%u, %u, %u]\n", uint_array[0], uint_array[1], uint_array[2]);

    uint8_array[0] = 100;
    uint8_array[1] = 150;
    uint8_array[2] = 200;
    printf("uint8 array: [%u, %u, %u]\n", uint8_array[0], uint8_array[1], uint8_array[2]);

    uint16_array[0] = 1000;
    uint16_array[1] = 2000;
    uint16_array[2] = 3000;
    printf("uint16 array: [%u, %u, %u]\n", uint16_array[0], uint16_array[1], uint16_array[2]);

    uint32_array[0] = 100000;
    uint32_array[1] = 200000;
    uint32_array[2] = 300000;
    printf("uint32 array: [%u, %u, %u]\n", uint32_array[0], uint32_array[1], uint32_array[2]);

    uint64_array[0] = 1000000;
    uint64_array[1] = 2000000;
    uint64_array[2] = 3000000;
    printf("uint64 array: [%llu, %llu, %llu]\n",
           (unsigned long long)uint64_array[0],
           (unsigned long long)uint64_array[1],
           (unsigned long long)uint64_array[2]);

    byte_array[0] = 'A';
    byte_array[1] = 'B';
    byte_array[2] = 'C';
    printf("byte array: [%c, %c, %c]\n", byte_array[0], byte_array[1], byte_array[2]);
}

void demo_float_arrays(void) {
    printf("\n-- Float arrays --\n");

    float   float_array[3];
    float   float32_array[3];
    double  float64_array[3];

    float_array[0] = 1.1f;
    float_array[1] = 2.2f;
    float_array[2] = 3.3f;
    printf("float array: [%.2f, %.2f, %.2f]\n", float_array[0], float_array[1], float_array[2]);

    float32_array[0] = 1.25f;
    float32_array[1] = 2.75f;
    float32_array[2] = 3.125f;
    printf("float32 array: [%.3f, %.3f, %.3f]\n", float32_array[0], float32_array[1], float32_array[2]);

    float64_array[0] = 1.123456;
    float64_array[1] = 2.789012;
    float64_array[2] = 3.456789;
    printf("float64 array: [%.6f, %.6f, %.6f]\n", float64_array[0], float64_array[1], float64_array[2]);
}

void demo_bool_arrays(void) {
    printf("\n-- Bool arrays --\n");

    bool bool_array[4];
    bool_array[0] = true;
    bool_array[1] = false;
    bool_array[2] = true;
    bool_array[3] = false;

    printf("bool array: [%s, %s, %s, %s]\n",
           bool_array[0] ? "true" : "false",
           bool_array[1] ? "true" : "false",
           bool_array[2] ? "true" : "false",
           bool_array[3] ? "true" : "false");
}

void demo_pointer_arrays(void) {
    printf("\n-- Pointer arrays --\n");

    int values[3];
    int *int_ptr_array[3];
    unsigned char *string_array[3];
    void *void_ptr_array[3];

    values[0] = 42;
    values[1] = 84;
    values[2] = 126;

    int_ptr_array[0] = &values[0];
    int_ptr_array[1] = &values[1];
    int_ptr_array[2] = &values[2];

    printf("int* array dereferenced: [%d, %d, %d]\n",
           *int_ptr_array[0], *int_ptr_array[1], *int_ptr_array[2]);

    string_array[0] = (unsigned char *)"Hello";
    string_array[1] = (unsigned char *)"World";
    string_array[2] = (unsigned char *)"GBC";

    printf("string array: [\"%s\", \"%s\", \"%s\"]\n",
           string_array[0], string_array[1], string_array[2]);

    void_ptr_array[0] = &values[0];
    void_ptr_array[1] = string_array[0];
    void_ptr_array[2] = int_ptr_array;
}

void demo_struct_arrays(void) {
    printf("\n-- Struct arrays --\n");

    Point point_array[3];

    point_array[0].x = 10;  point_array[0].y = 20;  point_array[0].name = (unsigned char *)"Origin";
    point_array[1].x = 100; point_array[1].y = 200; point_array[1].name = (unsigned char *)"Point A";
    point_array[2].x = -50; point_array[2].y = 75;  point_array[2].name = (unsigned char *)"Point B";

    printf("Point array:\n");
    int i = 0;
    while (i < 3) {
        printf("  [%d]: (%d, %d) \"%s\"\n", i,
               point_array[i].x, point_array[i].y, point_array[i].name);
        i = i + 1;
    }
}

void demo_enum_arrays(void) {
    printf("\n-- Enum arrays --\n");

    Color color_array[4];
    color_array[0] = RED;
    color_array[1] = GREEN;
    color_array[2] = BLUE;
    color_array[3] = YELLOW;

    printf("Color array:\n");
    int i = 0;
    while (i < 4) {
        const char *color_name;
        switch (color_array[i]) {
            case RED:    color_name = "RED"; break;
            case GREEN:  color_name = "GREEN"; break;
            case BLUE:   color_name = "BLUE"; break;
            case YELLOW: color_name = "YELLOW"; break;
            default:     color_name = "UNKNOWN";
        }
        printf("  [%d]: %s (%d)\n", i, color_name, color_array[i]);
        i = i + 1;
    }
}

void demo_struct_pointer_arrays(void) {
    printf("\n-- Struct pointer arrays (dynamic) --\n");

    Point *point_ptr_array[2];
    point_ptr_array[0] = malloc(sizeof(Point));
    point_ptr_array[1] = malloc(sizeof(Point));

    point_ptr_array[0]->x = 300;
    point_ptr_array[0]->y = 400;
    point_ptr_array[0]->name = (unsigned char *)"Dynamic A";

    point_ptr_array[1]->x = -150;
    point_ptr_array[1]->y = 250;
    point_ptr_array[1]->name = (unsigned char *)"Dynamic B";

    printf("Dynamic Point* array:\n");
    int i = 0;
    while (i < 2) {
        printf("  [%d]: (%d, %d) \"%s\"\n", i,
               point_ptr_array[i]->x, point_ptr_array[i]->y, point_ptr_array[i]->name);
        i = i + 1;
    }

    free(point_ptr_array[0]);
    free(point_ptr_array[1]);
}

int main(void) {
    printf("GBC array types test\n");
    printf("Quick run of arrays for core types.\n");

    demo_integer_arrays();
    demo_float_arrays();
    demo_bool_arrays();
    demo_pointer_arrays();
    demo_struct_arrays();
    demo_enum_arrays();
    demo_struct_pointer_arrays();

    printf("\nDone.\n");
    return 0;
}
