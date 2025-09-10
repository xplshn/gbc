main() {
    extrn puts;
    auto x; x = 2;
    switch (x) {
        case 1:
            puts("one");
            exit(1);
        case 2:
            puts("two");
            exit(0);
        default:
            puts("other");
            exit(1);
    }
    return(0);
}
