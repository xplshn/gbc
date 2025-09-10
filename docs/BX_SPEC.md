# The Bx Language Specification

## Introduction

Bx is a modern extension of the B programming language, preserving the elegant simplicity of Dennis Ritchie's original implementation while adding essential features for contemporary systems programming. Unlike B, Bx introduces an optional and backwards compatible type system, structured data types, floating-point arithmetic, and modern syntax conventions.

Bx maintains full backward compatibility with B programs while offering opt-in enhancements through feature flags.

## Language Philosophy

Bx follows these core principles:

1. **Backward Compatibility**: All valid B programs remain valid in Bx
2. **Optional Enhancement**: Modern features are opt-in via feature flags
3. **Simplicity**: Minimal syntax with maximum expressiveness
4. **Systems Programming**: Direct memory manipulation and efficient compilation

## Type System

### Overview

The Bx type system is optional and backward-compatible. When the `typed` feature is disabled, Bx behaves exactly like B with untyped word values. When enabled, Bx provides static type checking while preserving B's flexibility.

### Primitive Types

Bx supports a comprehensive set of primitive types:

```bx
// Integer types
int        // Platform word size (32 or 64 bits)
uint       // Unsigned platform word size
int8       // 8-bit signed integer
uint8      // 8-bit unsigned integer  
int16      // 16-bit signed integer
uint16     // 16-bit unsigned integer
int32      // 32-bit signed integer
uint32     // 32-bit unsigned integer
int64      // 64-bit signed integer
uint64     // 64-bit unsigned integer
byte       // Alias for uint8

// Floating-point types (requires float feature)
float      // Platform-native floating point
float32    // 32-bit IEEE 754 floating point
float64    // 64-bit IEEE 754 floating point

// Other types
bool       // Boolean (true/false)
string     // String type, alias to byte*
void       // Absence of value
any        // Type that matches anything
```

### Type Declarations

Variables can be declared with explicit types:

```bx
int32 counter = 0;
float64 pi = 3.141592653589793;
bool is_valid = true;
string name = "Bx Language";
```

### Pointer Types

Pointers are declared using the `*` suffix:

```bx
int *ptr;           // Pointer to int
byte *buffer;       // Pointer to byte
float32 *matrix;    // Pointer to float32
```

Pointer arithmetic follows C conventions:

```bx
int arr[10];
int *p = arr;
p = p + 1;          // Points to arr[1]
*p = 42;            // Sets arr[1] = 42
```

### Array Types

Arrays can be declared in two forms:

```bx
// Fixed-size arrays
int numbers[100];           // 100 integers
byte buffer[1024];          // 1024 bytes

// Dynamic arrays (slice notation)
[]int dynamic_array;        // Pointer to int with length semantics
[]byte text_buffer;         // Pointer to bytes
```

Fixed arrays are allocated on the stack, while dynamic arrays typically point to heap-allocated memory.

## Memory Layout and Alignment

### Word Size and Alignment

Bx follows platform-specific alignment requirements:

| Architecture | Word Size | Stack Alignment | Pointer Size |
|-------------|-----------|-----------------|--------------|
| x86_64      | 8 bytes   | 16 bytes        | 8 bytes      |
| i386        | 4 bytes   | 8 bytes         | 4 bytes      |
| ARM64       | 8 bytes   | 16 bytes        | 8 bytes      |
| ARM         | 4 bytes   | 8 bytes         | 4 bytes      |
| RISC-V 64   | 8 bytes   | 16 bytes        | 8 bytes      |

### Structure Layout

Structures are laid out sequentially in memory with appropriate padding for alignment:

```bx
type struct Point {
    x, y int32;         // 8 bytes total
};

type struct Mixed {
    flag bool;          // 1 byte
    // 3 bytes padding
    value int32;        // 4 bytes
    data int64;         // 8 bytes
    // Total: 16 bytes
};
```

The compiler automatically inserts padding to ensure proper alignment of structure members.

### Pointer Representation

Pointers are represented as machine addresses. On 64-bit systems, pointers are 8 bytes; on 32-bit systems, they are 4 bytes. Null pointers are represented as zero values.

## Structured Data Types

### Structures

Structures group related data under a single name:

```bx
type struct Person {
    name string;
    age int32;
    height float32;
};

Person john = Person{
    name: "John Doe",
    age: 30,
    height: 5.9
};

// Access members
printf("Name: %s, Age: %d\n", john.name, john.age);
```

#### Structure Literals

Structures can be initialized using literal syntax:

```bx
// Named field initialization
Point p1 = Point{x: 10, y: 20};

// Positional initialization (fields must be in order and all must be the same type)
Point p2 = Point{15, 25};

// Partial initialization (remaining fields are zero or nil)
Point p3 = Point{x: 5};  // y is 0
```

#### Nested Structures

Structures can contain other structures:

```bx
type struct Rectangle {
    top_left Point;
    bottom_right Point;
};

Rectangle rect = Rectangle{
    top_left: Point{0, 0},
    bottom_right: Point{100, 50}
};

// Access nested members
int width = rect.bottom_right.x - rect.top_left.x;
```

### Enumerations

Enumerations define named integer constants:

```bx
type enum Color {
    RED,        // 0
    GREEN,      // 1
    BLUE        // 2
};

type enum Status {
    OK = 0,
    ERROR = -1,
    PENDING = 1
};

Color background = RED;
Status result = OK;
```

Enumerations are strongly typed, preventing accidental mixing of different enum types.

## Literals and Escape Sequences

### String and Character Literals

Bx supports both string literals (enclosed in double quotes) and character literals (enclosed in single quotes):

```bx
string message = "Hello, World!";
int newline_char = '\n';
```

### Escape Sequences

Arbitrary character values can be encoded with escape sequences and used in string or character literals. Bx supports both C-style and B-style escape sequences, which can be enabled simultaneously through feature flags.

There are four different formats for arbitrary character values:

- `\x` or `*x` followed by exactly two hexadecimal digits
- `\` or `*` followed by exactly three octal digits  
- `\u` or `*u` followed by exactly four hexadecimal digits
- `\U` or `*U` followed by exactly eight hexadecimal digits

where the escapes `\u`/`*u` and `\U`/`*U` represent Unicode code points.

The following special escape values are also available:

| Value | Description |
|-------|-------------|
| `\a` or `*a` | Alert or bell |
| `\b` or `*b` | Backspace |
| `\\` or `**` | Backslash or asterisk |
| `\t` or `*t` | Horizontal tab |
| `\n` or `*n` | Line feed or newline |
| `\f` or `*f` | Form feed |
| `\r` or `*r` | Carriage return |
| `\v` or `*v` | Vertical tab |
| `\'` or `*'` | Single quote (only in character literals) |
| `\"` or `*"` | Double quote (only in string literals) |
| `\e` or `*e` | End-of-file |
| `\0` or `*0` | Null character |
| `\(` or `*(` | Left brace `{` |
| `\)` or `*)` | Right brace `}` |

#### Usage Examples

```bx
// C-style escapes (when -Fc-esc is enabled)
string c_style = "Line 1\nLine 2\tTabbed\x41\101";  // Mixed hex, octal
int escape_char = '\033';                            // Octal ESC character

// B-style escapes (when -Fb-esc is enabled)
string b_style = "Line 1*nLine 2*tTabbed*x42";      // B-style newline, tab, hex
string braces = "Function *( body *)";               // B-style braces

// Both styles can be mixed when both are enabled
string mixed = "C-style: \n B-style: *n";           // Both newlines
string unicode = "Unicode: \u0041 or *u0041";       // Both Unicode escapes
```

#### Feature Flags

Escape sequence behavior is controlled through compiler feature flags:

| Flag | Description |
|------|-------------|
| `-Fc-esc` | Enable C-style escape sequences using `\` prefix (default: enabled) |
| `-Wc-esc` | Enable warnings for C-style escape sequences |
| `-Fb-esc` | Enable B-style escape sequences using `*` prefix (default: disabled) |
| `-Wb-esc` | Enable warnings for B-style escape sequences |

Both C-style and B-style escape sequences support the same set of escape values and can be enabled simultaneously.

```bash
# Enable both escape styles with warnings
gbc -Fc-esc -Fb-esc -Wc-esc -Wb-esc program.bx

# Use only B-style escapes
gbc -Fno-c-esc -Fb-esc program.bx
```

## Operators and Expressions

### Arithmetic Operators

Bx supports both B (you must enable support for this legacy feature (`-Fb-ops`), or use `-std=B`) and C operators:

```bx
// Basic arithmetic
int a = 10 + 5;     // Addition
int b = 10 - 5;     // Subtraction
int c = 10 * 5;     // Multiplication
int d = 10 / 5;     // Division
int e = 10 % 3;     // Modulus

// Compound assignment (C-style)
a += 5;             // a = a + 5
b -= 3;             // b = b - 3
c *= 2;             // c = c * 2

// B-style            (deprecated, warns when used)
a =+ 5;             // Equivalent to a += 5
b =- 3;             // Equivalent to b -= 3
```

### Bitwise Operators

```bx
int flags = 0x0F;
flags = flags & 0x07;   // Bitwise AND
flags = flags | 0x08;   // Bitwise OR
flags = flags ^ 0x04;   // Bitwise XOR
flags = flags << 1;     // Left shift
flags = flags >> 1;     // Right shift
```

### Comparison and Logical Operators

```bx
bool result;
result = (a == b);      // Equality
result = (a != b);      // Inequality
result = (a < b);       // Less than
result = (a <= b);      // Less than or equal
result = (a > b);       // Greater than
result = (a >= b);      // Greater than or equal

result = (a && b);      // Logical AND
result = (a || b);      // Logical OR
result = !a;            // Logical NOT
```

### Pointer and Array Operators

```bx
int arr[10] = {1, 2, 3, 4, 5};
int *ptr = arr;

// Indirection and address-of
int value = *ptr;       // Dereference pointer
ptr = &arr[5];          // Address of arr[5]

// Array subscripting
arr[0] = 100;           // Set first element
int first = arr[0];     // Get first element

// Pointer arithmetic
ptr++;                  // Move to next element
ptr += 5;               // Move 5 elements forward
```

### Ternary Operator

The conditional operator provides concise conditional expressions:

```bx
int max_value = (a > b) ? a : b;
string message = (count > 0) ? "items found" : "no items";
```

## Control Flow

### Conditional Statements

```bx
// Simple if statement
if (condition) {
    // statements
}

// If-else
if (x > 0) {
    printf("Positive\n");
} else if (x < 0) {
    printf("Negative\n");
} else {
    printf("Zero\n");
}
```

### Loop Constructs

#### While Loops

```bx
int i = 0;
while (i < 10) {
    printf("%d\n", i);
    i++;
}
```

Note: Bx primarily uses while loops for iteration. Traditional for loops are not supported. But they will be like Go's once supported.

### Switch Statements

Switch statements support conditions, literals and enum values:

```bx
switch (value) {
    case 1:
        printf("One\n");
        break;
    case 2:
    case 3:
        printf("Two or Three\n");
        break;
    case 4, 5:
        printf("4 or 5");
        break;
    default:
        printf("Other\n");
        break;
}
```

### Loop Control

```bx
while (condition) {
    if (skip_condition) {
        continue;       // Skip to next iteration
    }
    
    if (exit_condition) {
        break;          // Exit loop
    }
    
    // Regular processing
}
```

### Goto Statements

Bx supports goto for low-level control flow:

```bx
start:
    if (condition) {
        goto end;
    }
    // processing
    goto start;

end:
    return (result);
```

## Functions

### Function Declarations

Functions can be declared with or without type annotations:

```bx
// Typed function declaration
int32 add(a, b int32) {
    return (a + b);
}

// Mixed parameter types
void process_data(name string, count int, factor float32) {
    // Implementation
}

// Untyped (B-style)
add(a, b) {
    return (a + b);
}

// Void function
void print_message(msg string) {
    printf("%s\n", msg);
}
```

### External Functions

External functions are declared using the `extrn` keyword:

```bx
extrn printf, malloc, free;

// Typed external declarations
int extrn strlen;
void* extrn malloc;
void extrn free;
tm* extrn localtime; // see ./examples/cal.bx for an example
```

### Function Pointers

Functions can be treated as first-class values:

```bx
int (*operation)(int, int) = add;
int result = operation(5, 3);   // Calls add(5, 3)
```

### Variadic Functions

Functions can accept variable numbers of arguments:

```bx
void log_message(string format, ...) {
    // Impl...
}
```

## Directives and Feature Control

### Inline Directives

Bx supports inline directives for fine-grained feature control:

```bx
// [b]: requires: -Ftyped -Fno-strict-decl
auto value = get_untyped_value();

// [b]: requires: -Wno-type
mixed_operation(int_var, float_var);
```

### Common Feature Flags

| Flag | Description | Default |
|------|-------------|---------|
| `typed` | Enable type system | On |
| `float` | Enable floating-point | On |
| `c-comments` | Allow // comments | On |
| `c-ops` | Use C-style operators | On |
| `short-decl` | Enable := syntax | On |
| `continue` | Allow continue statement | On |
| `strict-decl` | Require initialization | Off |

### Warning Control

```bx
// Disable specific warnings for a section
// [b]: requires: -Wno-type -Wno-implicit-decl
legacy_code_section();

// Enable pedantic warnings
// [b]: requires: -Wextra -Wpedantic
critical_function();
```

## Low-Level Details

### Calling Conventions

Bx follows platform-specific calling conventions:

- **System V AMD64 ABI** (i.e Linux)
- **Microsoft x64 ABI** on Windows
- **AAPCS** on ARM platforms

Function arguments are passed in registers when possible, with overflow on the stack.

### Stack Frame Layout

```
Higher addresses
+------------------+
| Return address   |
+------------------+
| Saved registers  |
+------------------+
| Local variables  |
+------------------+
| Spill area       |
+------------------+
Lower addresses
```

### Memory Model

Bx uses a simple memory model:
- Global variables are allocated in the data segment
- Local variables use stack allocation
- Dynamic allocation uses heap (via `malloc`/`free`)
- Pointer arithmetic follows byte addressing

### Floating-Point Representation

Bx uses IEEE 754 standard for floating-point numbers:
- `float32`: 32-bit single precision
- `float64`: 64-bit double precision
- Platform `float` maps to the type that matches the machine's word size

## Comparison with Classical B

| Feature | Classical B | Bx |
|---------|------------|-----|
| Types | Untyped words | Optional type system |
| Operators | `=+`, `=-`, etc. | `+=`, `-=`, etc. (C-style) |
| Comments | `/* */` only | `//` and `/* */` |
| Data structures | Arrays only | Arrays, structs, enums |
| Floating-point | Not supported | Full IEEE 754 support |
| Control flow | Basic | Enhanced with `continue` |

## Implementation Notes

### Compiler Architecture

The GBC compiler implements Bx as a superset of B using a multi-pass approach:

1. **Lexical Analysis**: Tokenizes source with feature-aware scanning
2. **Parsing**: Builds AST with optional type annotations  
3. **Type Checking**: Optional pass for type validation
4. **Code Generation**: Emits QBE intermediate representation
5. **Backend**: QBE/LLVM/etc handles optimization and native code generation

