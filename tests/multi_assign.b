// Test multi-assignment functionality
assert_equal(actual, expected, message) {
    extrn printf, abort;
    printf("%s: ", message);
    if (actual != expected) {
        printf("FAIL\n");
        abort();
    } else {
        printf("OK\n");
    }
}

main() {
    extrn assert_equal;
    
    // Test 1: Simple two-variable assignment
    auto a, b;
    a, b = 10, 20;
    assert_equal(a, 10, "a, b = 10, 20; a == 10");
    assert_equal(b, 20, "a, b = 10, 20; b == 20");
    
    // Test 2: Three-variable assignment
    auto x, y, z;
    x, y, z = 100, 200, 300;
    assert_equal(x, 100, "x, y, z = 100, 200, 300; x == 100");
    assert_equal(y, 200, "x, y, z = 100, 200, 300; y == 200");
    assert_equal(z, 300, "x, y, z = 100, 200, 300; z == 300");
    
    // Test 3: Array element assignment
    auto arr1 3, arr2 3;
    arr1[0], arr2[0] = 50, 60;
    assert_equal(arr1[0], 50, "arr1[0], arr2[0] = 50, 60; arr1[0] == 50");
    assert_equal(arr2[0], 60, "arr1[0], arr2[0] = 50, 60; arr2[0] == 60");
    
    // Test 4: Mixed lvalue types
    auto p, q, mixed 2;
    p, mixed[1], q = 1000, 2000, 3000;
    assert_equal(p, 1000, "p, mixed[1], q = 1000, 2000, 3000; p == 1000");
    assert_equal(mixed[1], 2000, "p, mixed[1], q = 1000, 2000, 3000; mixed[1] == 2000");
    assert_equal(q, 3000, "p, mixed[1], q = 1000, 2000, 3000; q == 3000");
    
    return(0);
}