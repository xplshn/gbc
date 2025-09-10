### [Module-Level Inline Assembly](https://llvm.org/docs/LangRef.html#id2008)[¶](https://llvm.org/docs/LangRef.html#module-level-inline-assembly "Link to this heading")

Modules may contain “module-level inline asm” blocks, which corresponds to the GCC “file scope inline asm” blocks. These blocks are internally concatenated by LLVM and treated as a single unit, but may be separated in the `.ll` file if desired. The syntax is very simple:

```
<span></span><span class="k">module</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="s">"inline asm code goes here"</span>
<span class="k">module</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="s">"more can go here"</span>
```

The strings can contain any character by escaping non-printable characters. The escape sequence used is simply “\\xx” where “xx” is the two digit hex code for the number.

Note that the assembly string _must_ be parseable by LLVM’s integrated assembler (unless it is disabled), even when emitting a `.s` file.

### [Data Layout](https://llvm.org/docs/LangRef.html#id2009)[¶](https://llvm.org/docs/LangRef.html#data-layout "Link to this heading")

A module may specify a target-specific data layout string that specifies how data is to be laid out in memory. The syntax for the data layout is simply:

```
<span></span><span class="k">target</span><span class="w"> </span><span class="k">datalayout</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="s">"layout specification"</span>
```

The _layout specification_ consists of a list of specifications separated by the minus sign character (‘-‘). Each specification starts with a letter and may include other information after the letter to define some aspect of the data layout. The specifications accepted are as follows:

`E`

Specifies that the target lays out data in big-endian form. That is, the bits with the most significance have the lowest address location.

`e`

Specifies that the target lays out data in little-endian form. That is, the bits with the least significance have the lowest address location.

`S<size>`

Specifies the natural alignment of the stack in bits. Alignment promotion of stack variables is limited to the natural stack alignment to avoid dynamic stack realignment. If omitted, the natural stack alignment defaults to “unspecified”, which does not prevent any alignment promotions.

`P<address space>`

Specifies the address space that corresponds to program memory. Harvard architectures can use this to specify what space LLVM should place things such as functions into. If omitted, the program memory space defaults to the default address space of 0, which corresponds to a Von Neumann architecture that has code and data in the same space.

`G<address space>`

Specifies the address space to be used by default when creating global variables. If omitted, the globals address space defaults to the default address space 0. Note: variable declarations without an address space are always created in address space 0, this property only affects the default value to be used when creating globals without additional contextual information (e.g. in LLVM passes).

`A<address space>`

Specifies the address space of objects created by ‘`alloca`’. Defaults to the default address space of 0.

`p[n]:<size>:<abi>[:<pref>[:<idx>]]`

This specifies the properties of a pointer in address space `n`. The `<size>` parameter specifies the size of the bitwise representation. For [non-integral pointers](https://llvm.org/docs/LangRef.html#nointptrtype) the representation size may be larger than the address width of the underlying address space (e.g. to accommodate additional metadata). The alignment requirements are specified via the `<abi>` and `<pref>`erred alignments parameters. The fourth parameter `<idx>` is the size of the index that used for address calculations such as [getelementptr](https://llvm.org/docs/LangRef.html#i-getelementptr). It must be less than or equal to the pointer size. If not specified, the default index size is equal to the pointer size. The index size also specifies the width of addresses in this address space. All sizes are in bits. The address space, `n`, is optional, and if not specified, denotes the default address space 0. The value of `n` must be in the range \[1,2^24).

`i<size>:<abi>[:<pref>]`

This specifies the alignment for an integer type of a given bit `<size>`. The value of `<size>` must be in the range \[1,2^24). For `i8`, the `<abi>` value must equal 8, that is, `i8` must be naturally aligned.

`v<size>:<abi>[:<pref>]`

This specifies the alignment for a vector type of a given bit `<size>`. The value of `<size>` must be in the range \[1,2^24).

`f<size>:<abi>[:<pref>]`

This specifies the alignment for a floating-point type of a given bit `<size>`. Only values of `<size>` that are supported by the target will work. 32 (float) and 64 (double) are supported on all targets; 80 or 128 (different flavors of long double) are also supported on some targets. The value of `<size>` must be in the range \[1,2^24).

`a:<abi>[:<pref>]`

This specifies the alignment for an object of aggregate type. In addition to the usual requirements for alignment values, the value of `<abi>` can also be zero, which means one byte alignment.

`F<type><abi>`

This specifies the alignment for function pointers. The options for `<type>` are:

-   `i`: The alignment of function pointers is independent of the alignment of functions, and is a multiple of `<abi>`.
    
-   `n`: The alignment of function pointers is a multiple of the explicit alignment specified on the function, and is a multiple of `<abi>`.
    

`m:<mangling>`

If present, specifies that llvm names are mangled in the output. Symbols prefixed with the mangling escape character `\01` are passed through directly to the assembler without the escape character. The mangling style options are

-   `e`: ELF mangling: Private symbols get a `.L` prefix.
    
-   `l`: GOFF mangling: Private symbols get a `@` prefix.
    
-   `m`: Mips mangling: Private symbols get a `$` prefix.
    
-   `o`: Mach-O mangling: Private symbols get `L` prefix. Other symbols get a `_` prefix.
    
-   `x`: Windows x86 COFF mangling: Private symbols get the usual prefix. Regular C symbols get a `_` prefix. Functions with `__stdcall`, `__fastcall`, and `__vectorcall` have custom mangling that appends `@N` where N is the number of bytes used to pass parameters. C++ symbols starting with `?` are not mangled in any way.
    
-   `w`: Windows COFF mangling: Similar to `x`, except that normal C symbols do not receive a `_` prefix.
    
-   `a`: XCOFF mangling: Private symbols get a `L..` prefix.
    

`n<size1>:<size2>:<size3>...`

This specifies a set of native integer widths for the target CPU in bits. For example, it might contain `n32` for 32-bit PowerPC, `n32:64` for PowerPC 64, or `n8:16:32:64` for X86-64. Elements of this set are considered to support most general arithmetic operations efficiently.

`ni:<address space0>:<address space1>:<address space2>...`

This specifies pointer types with the specified address spaces as [Non-Integral Pointer Type](https://llvm.org/docs/LangRef.html#nointptrtype) s. The `0` address space cannot be specified as non-integral.

`<abi>` is a lower bound on what is required for a type to be considered aligned. This is used in various places, such as:

-   The alignment for loads and stores if none is explicitly given.
    
-   The alignment used to compute struct layout.
    
-   The alignment used to compute allocation sizes and thus `getelementptr` offsets.
    
-   The alignment below which accesses are considered underaligned.
    

`<pref>` allows providing a more optimal alignment that should be used when possible, primarily for `alloca` and the alignment of global variables. It is an optional value that must be greater than or equal to `<abi>`. If omitted, the preceding `:` should also be omitted and `<pref>` will be equal to `<abi>`.

Unless explicitly stated otherwise, every alignment specification is provided in bits and must be in the range \[1,2^16). The value must be a power of two times the width of a byte (i.e., `align = 8 * 2^N`).

When constructing the data layout for a given target, LLVM starts with a default set of specifications which are then (possibly) overridden by the specifications in the `datalayout` keyword. The default specifications are given in this list:

-   `e` - little endian
    
-   `p:64:64:64` - 64-bit pointers with 64-bit alignment.
    
-   `p[n]:64:64:64` - Other address spaces are assumed to be the same as the default address space.
    
-   `S0` - natural stack alignment is unspecified
    
-   `i1:8:8` - i1 is 8-bit (byte) aligned
    
-   `i8:8:8` - i8 is 8-bit (byte) aligned as mandated
    
-   `i16:16:16` - i16 is 16-bit aligned
    
-   `i32:32:32` - i32 is 32-bit aligned
    
-   `i64:32:64` - i64 has ABI alignment of 32-bits but preferred alignment of 64-bits
    
-   `f16:16:16` - half is 16-bit aligned
    
-   `f32:32:32` - float is 32-bit aligned
    
-   `f64:64:64` - double is 64-bit aligned
    
-   `f128:128:128` - quad is 128-bit aligned
    
-   `v64:64:64` - 64-bit vector is 64-bit aligned
    
-   `v128:128:128` - 128-bit vector is 128-bit aligned
    
-   `a:0:64` - aggregates are 64-bit aligned
    

When LLVM is determining the alignment for a given type, it uses the following rules:

1.  If the type sought is an exact match for one of the specifications, that specification is used.
    
2.  If no match is found, and the type sought is an integer type, then the smallest integer type that is larger than the bitwidth of the sought type is used. If none of the specifications are larger than the bitwidth then the largest integer type is used. For example, given the default specifications above, the i7 type will use the alignment of i8 (next largest) while both i65 and i256 will use the alignment of i64 (largest specified).
    

The function of the data layout string may not be what you expect. Notably, this is not a specification from the frontend of what alignment the code generator should use.

Instead, if specified, the target data layout is required to match what the ultimate _code generator_ expects. This string is used by the mid-level optimizers to improve code, and this only works if it matches what the ultimate code generator uses. There is no way to generate IR that does not embed this target-specific detail into the IR. If you don’t specify the string, the default specifications will be used to generate a Data Layout and the optimization phases will operate accordingly and introduce target specificity into the IR with respect to these default specifications.

### [Target Triple](https://llvm.org/docs/LangRef.html#id2010)[¶](https://llvm.org/docs/LangRef.html#target-triple "Link to this heading")

A module may specify a target triple string that describes the target host. The syntax for the target triple is simply:

```
<span></span><span class="k">target</span><span class="w"> </span><span class="k">triple</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="s">"x86_64-apple-macosx10.7.0"</span>
```

The _target triple_ string consists of a series of identifiers delimited by the minus sign character (‘-‘). The canonical forms are:

```
<span></span><span class="n">ARCHITECTURE</span><span class="o">-</span><span class="n">VENDOR</span><span class="o">-</span><span class="n">OPERATING_SYSTEM</span>
<span class="n">ARCHITECTURE</span><span class="o">-</span><span class="n">VENDOR</span><span class="o">-</span><span class="n">OPERATING_SYSTEM</span><span class="o">-</span><span class="n">ENVIRONMENT</span>
```

This information is passed along to the backend so that it generates code for the proper architecture. It’s possible to override this on the command line with the `-mtriple` command-line option.

### [Allocated Objects](https://llvm.org/docs/LangRef.html#id2011)[¶](https://llvm.org/docs/LangRef.html#allocated-objects "Link to this heading")

An allocated object, memory object, or simply object, is a region of a memory space that is reserved by a memory allocation such as [alloca](https://llvm.org/docs/LangRef.html#i-alloca), heap allocation calls, and global variable definitions. Once it is allocated, the bytes stored in the region can only be read or written through a pointer that is [based on](https://llvm.org/docs/LangRef.html#pointeraliasing) the allocation value. If a pointer that is not based on the object tries to read or write to the object, it is undefined behavior.

The following properties hold for all allocated objects, otherwise the behavior is undefined:

-   no allocated object may cross the unsigned address space boundary (including the pointer after the end of the object),
    
-   the size of all allocated objects must be non-negative and not exceed the largest signed integer that fits into the index type.
    

Allocated objects that are created with operations recognized by LLVM (such as [alloca](https://llvm.org/docs/LangRef.html#i-alloca), heap allocation functions marked as such, and global variables) may _not_ change their size. (`realloc`\-style operations do not change the size of an existing allocated object; instead, they create a new allocated object. Even if the object is at the same location as the old one, old pointers cannot be used to access this new object.) However, allocated objects can also be created by means not recognized by LLVM, e.g. by directly calling `mmap`. Those allocated objects are allowed to grow to the right (i.e., keeping the same base address, but increasing their size) while maintaining the validity of existing pointers, as long as they always satisfy the properties described above. Currently, allocated objects are not permitted to grow to the left or to shrink, nor can they have holes.

### [Object Lifetime](https://llvm.org/docs/LangRef.html#id2012)[¶](https://llvm.org/docs/LangRef.html#object-lifetime "Link to this heading")

A lifetime of an [allocated object](https://llvm.org/docs/LangRef.html#allocatedobjects) is a property that decides its accessibility. Unless stated otherwise, an allocated object is alive since its allocation, and dead after its deallocation. It is undefined behavior to access an allocated object that isn’t alive, but operations that don’t dereference it such as [getelementptr](https://llvm.org/docs/LangRef.html#i-getelementptr), [ptrtoint](https://llvm.org/docs/LangRef.html#i-ptrtoint) and [icmp](https://llvm.org/docs/LangRef.html#i-icmp) return a valid result. This explains code motion of these instructions across operations that impact the object’s lifetime. A stack object’s lifetime can be explicitly specified using [llvm.lifetime.start](https://llvm.org/docs/LangRef.html#int-lifestart) and [llvm.lifetime.end](https://llvm.org/docs/LangRef.html#int-lifeend) intrinsic function calls.

### [Pointer Aliasing Rules](https://llvm.org/docs/LangRef.html#id2013)[¶](https://llvm.org/docs/LangRef.html#pointer-aliasing-rules "Link to this heading")

Any memory access must be done through a pointer value associated with an address range of the memory access, otherwise the behavior is undefined. Pointer values are associated with address ranges according to the following rules:

-   A pointer value is associated with the addresses associated with any value it is _based_ on.
    
-   An address of a global variable is associated with the address range of the variable’s storage.
    
-   The result value of an allocation instruction is associated with the address range of the allocated storage.
    
-   A null pointer in the default address-space is associated with no address.
    
-   An [undef value](https://llvm.org/docs/LangRef.html#undefvalues) in _any_ address-space is associated with no address.
    
-   An integer constant other than zero or a pointer value returned from a function not defined within LLVM may be associated with address ranges allocated through mechanisms other than those provided by LLVM. Such ranges shall not overlap with any ranges of addresses allocated by mechanisms provided by LLVM.
    

A pointer value is _based_ on another pointer value according to the following rules:

-   A pointer value formed from a scalar `getelementptr` operation is _based_ on the pointer-typed operand of the `getelementptr`.
    
-   The pointer in lane _l_ of the result of a vector `getelementptr` operation is _based_ on the pointer in lane _l_ of the vector-of-pointers-typed operand of the `getelementptr`.
    
-   The result value of a `bitcast` is _based_ on the operand of the `bitcast`.
    
-   A pointer value formed by an `inttoptr` is _based_ on all pointer values that contribute (directly or indirectly) to the computation of the pointer’s value.
    
-   The “_based_ on” relationship is transitive.
    

Note that this definition of _“based”_ is intentionally similar to the definition of _“based”_ in C99, though it is slightly weaker.

LLVM IR does not associate types with memory. The result type of a `load` merely indicates the size and alignment of the memory from which to load, as well as the interpretation of the value. The first operand type of a `store` similarly only indicates the size and alignment of the store.

Consequently, type-based alias analysis, aka TBAA, aka `-fstrict-aliasing`, is not applicable to general unadorned LLVM IR. [Metadata](https://llvm.org/docs/LangRef.html#metadata) may be used to encode additional information which specialized optimization passes may use to implement type-based alias analysis.

### [Pointer Capture](https://llvm.org/docs/LangRef.html#id2014)[¶](https://llvm.org/docs/LangRef.html#pointer-capture "Link to this heading")

Given a function call and a pointer that is passed as an argument or stored in memory before the call, the call may capture two components of the pointer:

> -   The address of the pointer, which is its integral value. This also includes parts of the address or any information about the address, including the fact that it does not equal one specific value. We further distinguish whether only the fact that the address is/isn’t null is captured.
>     
> -   The provenance of the pointer, which is the ability to perform memory accesses through the pointer, in the sense of the [pointer aliasing rules](https://llvm.org/docs/LangRef.html#pointeraliasing). We further distinguish whether only read accesses are allowed, or both reads and writes.
>     

For example, the following function captures the address of `%a`, because it is compared to a pointer, leaking information about the identity of the pointer:

```
<span></span><span class="vg">@glb</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="m">0</span>

<span class="k">define</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%c</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">icmp</span><span class="w"> </span><span class="k">eq</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="vg">@glb</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="nv">%c</span>
<span class="p">}</span>
```

The function does not capture the provenance of the pointer, because the `icmp` instruction only operates on the pointer address. The following function captures both the address and provenance of the pointer, as both may be read from `@glb` after the function returns:

```
<span></span><span class="vg">@glb</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span>

<span class="k">define</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="k">void</span>
<span class="p">}</span>
```

The following function captures _neither_ the address nor the provenance of the pointer:

```
<span></span><span class="k">define</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%v</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">load</span><span class="w"> </span><span class="kt">i32</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">i32</span>
<span class="p">}</span>
```

While address capture includes uses of the address within the body of the function, provenance capture refers exclusively to the ability to perform accesses _after_ the function returns. Memory accesses within the function itself are not considered pointer captures.

We can further say that the capture only occurs through a specific location. In the following example, the pointer (both address and provenance) is captured through the return value only:

```
<span></span><span class="k">define</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%gep</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">getelementptr</span><span class="w"> </span><span class="kt">i8</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="m">4</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%gep</span>
<span class="p">}</span>
```

However, we always consider direct inspection of the pointer address (e.g. using `ptrtoint`) to be location-independent. The following example is _not_ considered a return-only capture, even though the `ptrtoint` ultimately only contributes to the return value:

```
<span></span><span class="vg">@lookup</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">constant</span><span class="w"> </span><span class="p">[</span><span class="m">4</span><span class="w"> </span><span class="k">x</span><span class="w"> </span><span class="kt">i8</span><span class="p">]</span><span class="w"> </span><span class="p">[</span><span class="kt">i8</span><span class="w"> </span><span class="m">0</span><span class="p">,</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="m">2</span><span class="p">,</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="m">3</span><span class="p">]</span>

<span class="k">define</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%a.addr</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">ptrtoint</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i64</span>
<span class="w">  </span><span class="nv">%mask</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">and</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%a.addr</span><span class="p">,</span><span class="w"> </span><span class="m">3</span>
<span class="w">  </span><span class="nv">%gep</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">getelementptr</span><span class="w"> </span><span class="kt">i8</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@lookup</span><span class="p">,</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%mask</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%gep</span>
<span class="p">}</span>
```

This definition is chosen to allow capture analysis to continue with the return value in the usual fashion.

The following describes possible ways to capture a pointer in more detail, where unqualified uses of the word “capture” refer to capturing both address and provenance.

1.  The call stores any bit of the pointer carrying information into a place, and the stored bits can be read from the place by the caller after this call exits.
    

```
<span></span><span class="vg">@glb</span><span class="w">  </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span>
<span class="vg">@glb2</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span>
<span class="vg">@glb3</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span>
<span class="vg">@glbi</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="m">0</span>

<span class="k">define</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%b</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%c</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%d</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%e</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb</span><span class="w"> </span><span class="c">; %a is captured by this call</span>

<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%b</span><span class="p">,</span><span class="w">   </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb2</span><span class="w"> </span><span class="c">; %b isn't captured because the stored value is overwritten by the store below</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb2</span>

<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%c</span><span class="p">,</span><span class="w">   </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb3</span>
<span class="w">  </span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="vg">@g</span><span class="p">()</span><span class="w"> </span><span class="c">; If @g makes a copy of %c that outlives this call (@f), %c is captured</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb3</span>

<span class="w">  </span><span class="nv">%i</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">ptrtoint</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%d</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i64</span>
<span class="w">  </span><span class="nv">%j</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">trunc</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%i</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i32</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="nv">%j</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glbi</span><span class="w"> </span><span class="c">; %d is captured</span>

<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%e</span><span class="w"> </span><span class="c">; %e is captured</span>
<span class="p">}</span>
```

2.  The call stores any bit of the pointer carrying information into a place, and the stored bits can be safely read from the place by another thread via synchronization.
    

```
<span></span><span class="vg">@lock</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="k">true</span>

<span class="k">define</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="k">atomic</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="k">false</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@lock</span><span class="w"> </span><span class="k">release</span><span class="w"> </span><span class="c">; %a is captured because another thread can safely read @glb</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="k">null</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@glb</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="k">void</span>
<span class="p">}</span>
```

3.  The call’s behavior depends on any bit of the pointer carrying information (address capture only).
    

```
<span></span><span class="vg">@glb</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="m">0</span>

<span class="k">define</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="vg">@f</span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%c</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">icmp</span><span class="w"> </span><span class="k">eq</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="vg">@glb</span>
<span class="w">  </span><span class="k">br</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="nv">%c</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%BB_EXIT</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%BB_CONTINUE</span><span class="w"> </span><span class="c">; captures address of %a only</span>
<span class="nl">BB_EXIT:</span>
<span class="w">  </span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="vg">@exit</span><span class="p">()</span>
<span class="w">  </span><span class="k">unreachable</span>
<span class="nl">BB_CONTINUE:</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="k">void</span>
<span class="p">}</span>
```

4.  The pointer is used as the pointer operand of a volatile access.
    

### [Volatile Memory Accesses](https://llvm.org/docs/LangRef.html#id2015)[¶](https://llvm.org/docs/LangRef.html#volatile-memory-accesses "Link to this heading")

Certain memory accesses, such as [load](https://llvm.org/docs/LangRef.html#i-load)’s, [store](https://llvm.org/docs/LangRef.html#i-store)’s, and [llvm.memcpy](https://llvm.org/docs/LangRef.html#int-memcpy)’s may be marked `volatile`. The optimizers must not change the number of volatile operations or change their order of execution relative to other volatile operations. The optimizers _may_ change the order of volatile operations relative to non-volatile operations. This is not Java’s “volatile” and has no cross-thread synchronization behavior.

A volatile load or store may have additional target-specific semantics. Any volatile operation can have side effects, and any volatile operation can read and/or modify state which is not accessible via a regular load or store in this module. Volatile operations may use addresses which do not point to memory (like MMIO registers). This means the compiler may not use a volatile operation to prove a non-volatile access to that address has defined behavior. This includes addresses typically forbidden, such as the pointer with bit-value 0.

The allowed side-effects for volatile accesses are limited. If a non-volatile store to a given address would be legal, a volatile operation may modify the memory at that address. A volatile operation may not modify any other memory accessible by the module being compiled. A volatile operation may not call any code in the current module.

In general (without target-specific context), the address space of a volatile operation may not be changed. Different address spaces may have different trapping behavior when dereferencing an invalid pointer.

The compiler may assume execution will continue after a volatile operation, so operations which modify memory or may have undefined behavior can be hoisted past a volatile operation.

As an exception to the preceding rule, the compiler may not assume execution will continue after a volatile store operation. This restriction is necessary to support the somewhat common pattern in C of intentionally storing to an invalid pointer to crash the program. In the future, it might make sense to allow frontends to control this behavior.

IR-level volatile loads and stores cannot safely be optimized into `llvm.memcpy` or `llvm.memmove` intrinsics even when those intrinsics are flagged volatile. Likewise, the backend should never split or merge target-legal volatile load/store instructions. Similarly, IR-level volatile loads and stores cannot change from integer to floating-point or vice versa.

Rationale

Platforms may rely on volatile loads and stores of natively supported data width to be executed as single instruction. For example, in C this holds for an l-value of volatile primitive type with native hardware support, but not necessarily for aggregate types. The frontend upholds these expectations, which are intentionally unspecified in the IR. The rules above ensure that IR transformations do not violate the frontend’s contract with the language.

### [Memory Model for Concurrent Operations](https://llvm.org/docs/LangRef.html#id2016)[¶](https://llvm.org/docs/LangRef.html#memory-model-for-concurrent-operations "Link to this heading")

The LLVM IR does not define any way to start parallel threads of execution or to register signal handlers. Nonetheless, there are platform-specific ways to create them, and we define LLVM IR’s behavior in their presence. This model is inspired by the C++ memory model.

For a more informal introduction to this model, see the [LLVM Atomic Instructions and Concurrency Guide](https://llvm.org/docs/LangRef.htmlAtomics.html).

We define a _happens-before_ partial order as the least partial order that

-   Is a superset of single-thread program order, and
    
-   When `a` _synchronizes-with_ `b`, includes an edge from `a` to `b`. _Synchronizes-with_ pairs are introduced by platform-specific techniques, like pthread locks, thread creation, thread joining, etc., and by atomic instructions. (See also [Atomic Memory Ordering Constraints](https://llvm.org/docs/LangRef.html#ordering)).
    

Note that program order does not introduce _happens-before_ edges between a thread and signals executing inside that thread.

Every (defined) read operation (load instructions, memcpy, atomic loads/read-modify-writes, etc.) R reads a series of bytes written by (defined) write operations (store instructions, atomic stores/read-modify-writes, memcpy, etc.). For the purposes of this section, initialized globals are considered to have a write of the initializer which is atomic and happens before any other read or write of the memory in question. For each byte of a read R, R<sub>byte</sub> may see any write to the same byte, except:

-   If write<sub>1</sub> happens before write<sub>2</sub>, and write<sub>2</sub> happens before R<sub>byte</sub>, then R<sub>byte</sub> does not see write<sub>1</sub>.
    
-   If R<sub>byte</sub> happens before write<sub>3</sub>, then R<sub>byte</sub> does not see write<sub>3</sub>.
    

Given that definition, R<sub>byte</sub> is defined as follows:

-   If R is volatile, the result is target-dependent. (Volatile is supposed to give guarantees which can support `sig_atomic_t` in C/C++, and may be used for accesses to addresses that do not behave like normal memory. It does not generally provide cross-thread synchronization.)
    
-   Otherwise, if there is no write to the same byte that happens before R<sub>byte</sub>, R<sub>byte</sub> returns `undef` for that byte.
    
-   Otherwise, if R<sub>byte</sub> may see exactly one write, R<sub>byte</sub> returns the value written by that write.
    
-   Otherwise, if R is atomic, and all the writes R<sub>byte</sub> may see are atomic, it chooses one of the values written. See the [Atomic Memory Ordering Constraints](https://llvm.org/docs/LangRef.html#ordering) section for additional constraints on how the choice is made.
    
-   Otherwise R<sub>byte</sub> returns `undef`.
    

R returns the value composed of the series of bytes it read. This implies that some bytes within the value may be `undef` **without** the entire value being `undef`. Note that this only defines the semantics of the operation; it doesn’t mean that targets will emit more than one instruction to read the series of bytes.

Note that in cases where none of the atomic intrinsics are used, this model places only one restriction on IR transformations on top of what is required for single-threaded execution: introducing a store to a byte which might not otherwise be stored is not allowed in general. (Specifically, in the case where another thread might write to and read from an address, introducing a store can change a load that may see exactly one write into a load that may see multiple writes.)

### [Atomic Memory Ordering Constraints](https://llvm.org/docs/LangRef.html#id2017)[¶](https://llvm.org/docs/LangRef.html#atomic-memory-ordering-constraints "Link to this heading")

Atomic instructions ([cmpxchg](https://llvm.org/docs/LangRef.html#i-cmpxchg), [atomicrmw](https://llvm.org/docs/LangRef.html#i-atomicrmw), [fence](https://llvm.org/docs/LangRef.html#i-fence), [atomic load](https://llvm.org/docs/LangRef.html#i-load), and [atomic store](https://llvm.org/docs/LangRef.html#i-store)) take ordering parameters that determine which other atomic instructions on the same address they _synchronize with_. These semantics implement the Java or C++ memory models; if these descriptions aren’t precise enough, check those specs (see spec references in the [atomics guide](https://llvm.org/docs/LangRef.htmlAtomics.html)). [fence](https://llvm.org/docs/LangRef.html#i-fence) instructions treat these orderings somewhat differently since they don’t take an address. See that instruction’s documentation for details.

For a simpler introduction to the ordering constraints, see the [LLVM Atomic Instructions and Concurrency Guide](https://llvm.org/docs/LangRef.htmlAtomics.html).

`unordered`

The set of values that can be read is governed by the happens-before partial order. A value cannot be read unless some operation wrote it. This is intended to provide a guarantee strong enough to model Java’s non-volatile shared variables. This ordering cannot be specified for read-modify-write operations; it is not strong enough to make them atomic in any interesting way.

`monotonic`

In addition to the guarantees of `unordered`, there is a single total order for modifications by `monotonic` operations on each address. All modification orders must be compatible with the happens-before order. There is no guarantee that the modification orders can be combined to a global total order for the whole program (and this often will not be possible). The read in an atomic read-modify-write operation ([cmpxchg](https://llvm.org/docs/LangRef.html#i-cmpxchg) and [atomicrmw](https://llvm.org/docs/LangRef.html#i-atomicrmw)) reads the value in the modification order immediately before the value it writes. If one atomic read happens before another atomic read of the same address, the later read must see the same value or a later value in the address’s modification order. This disallows reordering of `monotonic` (or stronger) operations on the same address. If an address is written `monotonic`\-ally by one thread, and other threads `monotonic`\-ally read that address repeatedly, the other threads must eventually see the write. This corresponds to the C/C++ `memory_order_relaxed`.

`acquire`

In addition to the guarantees of `monotonic`, a _synchronizes-with_ edge may be formed with a `release` operation. This is intended to model C/C++’s `memory_order_acquire`.

`release`

In addition to the guarantees of `monotonic`, if this operation writes a value which is subsequently read by an `acquire` operation, it _synchronizes-with_ that operation. Furthermore, this occurs even if the value written by a `release` operation has been modified by a read-modify-write operation before being read. (Such a set of operations comprises a _release sequence_). This corresponds to the C/C++ `memory_order_release`.

`acq_rel` (acquire+release)

Acts as both an `acquire` and `release` operation on its address. This corresponds to the C/C++ `memory_order_acq_rel`.

`seq_cst` (sequentially consistent)

In addition to the guarantees of `acq_rel` (`acquire` for an operation that only reads, `release` for an operation that only writes), there is a global total order on all sequentially-consistent operations on all addresses. Each sequentially-consistent read sees the last preceding write to the same address in this global order. This corresponds to the C/C++ `memory_order_seq_cst` and Java `volatile`.

Note: this global total order is _not_ guaranteed to be fully consistent with the _happens-before_ partial order if non-`seq_cst` accesses are involved. See the C++ standard [\[atomics.order\]](https://wg21.link/atomics.order) section for more details on the exact guarantees.

If an atomic operation is marked `syncscope("singlethread")`, it only _synchronizes with_ and only participates in the seq\_cst total orderings of other operations running in the same thread (for example, in signal handlers).

If an atomic operation is marked `syncscope("<target-scope>")`, where `<target-scope>` is a target-specific synchronization scope, then it is target dependent if it _synchronizes with_ and participates in the seq\_cst total orderings of other operations.

Otherwise, an atomic operation that is not marked `syncscope("singlethread")` or `syncscope("<target-scope>")` _synchronizes with_ and participates in the seq\_cst total orderings of other operations that are not marked `syncscope("singlethread")` or `syncscope("<target-scope>")`.

### [Floating-Point Environment](https://llvm.org/docs/LangRef.html#id2018)[¶](https://llvm.org/docs/LangRef.html#floating-point-environment "Link to this heading")

The default LLVM floating-point environment assumes that traps are disabled and status flags are not observable. Therefore, floating-point math operations do not have side effects and may be speculated freely. Results assume the round-to-nearest rounding mode, and subnormals are assumed to be preserved.

Running LLVM code in an environment where these assumptions are not met typically leads to undefined behavior. The `strictfp` and `denormal-fp-math` attributes as well as [Constrained Floating-Point Intrinsics](https://llvm.org/docs/LangRef.html#constrainedfp) can be used to weaken LLVM’s assumptions and ensure defined behavior in non-default floating-point environments; see their respective documentation for details.

### [Behavior of Floating-Point NaN values](https://llvm.org/docs/LangRef.html#id2019)[¶](https://llvm.org/docs/LangRef.html#behavior-of-floating-point-nan-values "Link to this heading")

A floating-point NaN value consists of a sign bit, a quiet/signaling bit, and a payload (which makes up the rest of the mantissa except for the quiet/signaling bit). LLVM assumes that the quiet/signaling bit being set to `1` indicates a quiet NaN (QNaN), and a value of `0` indicates a signaling NaN (SNaN). In the following we will hence just call it the “quiet bit”.

The representation bits of a floating-point value do not mutate arbitrarily; in particular, if there is no floating-point operation being performed, NaN signs, quiet bits, and payloads are preserved.

For the purpose of this section, `bitcast` as well as the following operations are not “floating-point math operations”: `fneg`, `llvm.fabs`, and `llvm.copysign`. These operations act directly on the underlying bit representation and never change anything except possibly for the sign bit.

Floating-point math operations that return a NaN are an exception from the general principle that LLVM implements IEEE-754 semantics. Unless specified otherwise, the following rules apply whenever the IEEE-754 semantics say that a NaN value is returned: the result has a non-deterministic sign; the quiet bit and payload are non-deterministically chosen from the following set of options:

-   The quiet bit is set and the payload is all-zero. (“Preferred NaN” case)
    
-   The quiet bit is set and the payload is copied from any input operand that is a NaN. (“Quieting NaN propagation” case)
    
-   The quiet bit and payload are copied from any input operand that is a NaN. (“Unchanged NaN propagation” case)
    
-   The quiet bit is set and the payload is picked from a target-specific set of “extra” possible NaN payloads. The set can depend on the input operand values. This set is empty on x86 and ARM, but can be non-empty on other architectures. (For instance, on wasm, if any input NaN does not have the preferred all-zero payload or any input NaN is an SNaN, then this set contains all possible payloads; otherwise, it is empty. On SPARC, this set consists of the all-one payload.)
    

In particular, if all input NaNs are quiet (or if there are no input NaNs), then the output NaN is definitely quiet. Signaling NaN outputs can only occur if they are provided as an input value. For example, “fmul SNaN, 1.0” may be simplified to SNaN rather than QNaN. Similarly, if all input NaNs are preferred (or if there are no input NaNs) and the target does not have any “extra” NaN payloads, then the output NaN is guaranteed to be preferred.

Floating-point math operations are allowed to treat all NaNs as if they were quiet NaNs. For example, “pow(1.0, SNaN)” may be simplified to 1.0.

Code that requires different behavior than this should use the [Constrained Floating-Point Intrinsics](https://llvm.org/docs/LangRef.html#constrainedfp). In particular, constrained intrinsics rule out the “Unchanged NaN propagation” case; they are guaranteed to return a QNaN.

Unfortunately, due to hard-or-impossible-to-fix issues, LLVM violates its own specification on some architectures:

-   x86-32 without SSE2 enabled may convert floating-point values to x86\_fp80 and back when performing floating-point math operations; this can lead to results with different precision than expected and it can alter NaN values. Since optimizations can make contradicting assumptions, this can lead to arbitrary miscompilations. See [issue #44218](https://github.com/llvm/llvm-project/issues/44218).
    
-   x86-32 (even with SSE2 enabled) may implicitly perform such a conversion on values returned from a function for some calling conventions. See [issue #66803](https://github.com/llvm/llvm-project/issues/66803).
    
-   Older MIPS versions use the opposite polarity for the quiet/signaling bit, and LLVM does not correctly represent this. See [issue #60796](https://github.com/llvm/llvm-project/issues/60796).
    

### [Floating-Point Semantics](https://llvm.org/docs/LangRef.html#id2020)[¶](https://llvm.org/docs/LangRef.html#floating-point-semantics "Link to this heading")

This section defines the semantics for core floating-point operations on types that use a format specified by IEEE-754. These types are: `half`, `float`, `double`, and `fp128`, which correspond to the binary16, binary32, binary64, and binary128 formats, respectively. The “core” operations are those defined in section 5 of IEEE-754, which all have corresponding LLVM operations.

The value returned by those operations matches that of the corresponding IEEE-754 operation executed in the [default LLVM floating-point environment](https://llvm.org/docs/LangRef.html#floatenv), except that the behavior of NaN results is instead [as specified here](https://llvm.org/docs/LangRef.html#floatnan). In particular, such a floating-point instruction returning a non-NaN value is guaranteed to always return the same bit-identical result on all machines and optimization levels.

This means that optimizations and backends may not change the observed bitwise result of these operations in any way (unless NaNs are returned), and frontends can rely on these operations providing correctly rounded results as described in the standard.

(Note that this is only about the value returned by these operations; see the [floating-point environment section](https://llvm.org/docs/LangRef.html#floatenv) regarding flags and exceptions.)

Various flags, attributes, and metadata can alter the behavior of these operations and thus make them not bit-identical across machines and optimization levels any more: most notably, the [fast-math flags](https://llvm.org/docs/LangRef.html#fastmath) as well as the [strictfp](https://llvm.org/docs/LangRef.html#strictfp) and [denormal-fp-math](https://llvm.org/docs/LangRef.html#denormal-fp-math) attributes and fpmath metadata <fpmath-metadata>. See their corresponding documentation for details.

### [Fast-Math Flags](https://llvm.org/docs/LangRef.html#id2021)[¶](https://llvm.org/docs/LangRef.html#fast-math-flags "Link to this heading")

LLVM IR floating-point operations ([fneg](https://llvm.org/docs/LangRef.html#i-fneg), [fadd](https://llvm.org/docs/LangRef.html#i-fadd), [fsub](https://llvm.org/docs/LangRef.html#i-fsub), [fmul](https://llvm.org/docs/LangRef.html#i-fmul), [fdiv](https://llvm.org/docs/LangRef.html#i-fdiv), [frem](https://llvm.org/docs/LangRef.html#i-frem), [fcmp](https://llvm.org/docs/LangRef.html#i-fcmp), [fptrunc](https://llvm.org/docs/LangRef.html#i-fptrunc), [fpext](https://llvm.org/docs/LangRef.html#i-fpext)), and [phi](https://llvm.org/docs/LangRef.html#i-phi), [select](https://llvm.org/docs/LangRef.html#i-select), or [call](https://llvm.org/docs/LangRef.html#i-call) instructions that return floating-point types may use the following flags to enable otherwise unsafe floating-point transformations.

`fast`

This flag is a shorthand for specifying all fast-math flags at once, and imparts no additional semantics from using all of them.

`nnan`

No NaNs - Allow optimizations to assume the arguments and result are not NaN. If an argument is a nan, or the result would be a nan, it produces a [poison value](https://llvm.org/docs/LangRef.html#poisonvalues) instead.

`ninf`

No Infs - Allow optimizations to assume the arguments and result are not +/-Inf. If an argument is +/-Inf, or the result would be +/-Inf, it produces a [poison value](https://llvm.org/docs/LangRef.html#poisonvalues) instead.

`nsz`

No Signed Zeros - Allow optimizations to treat the sign of a zero argument or zero result as insignificant. This does not imply that -0.0 is poison and/or guaranteed to not exist in the operation.

Note: For [phi](https://llvm.org/docs/LangRef.html#i-phi), [select](https://llvm.org/docs/LangRef.html#i-select), and [call](https://llvm.org/docs/LangRef.html#i-call) instructions, the following return types are considered to be floating-point types:

-   Floating-point scalar or vector types
    
-   Array types (nested to any depth) of floating-point scalar or vector types
    
-   Homogeneous literal struct types of floating-point scalar or vector types
    

#### [Rewrite-based flags](https://llvm.org/docs/LangRef.html#id2022)[¶](https://llvm.org/docs/LangRef.html#rewrite-based-flags "Link to this heading")

The following flags have rewrite-based semantics. These flags allow expressions, potentially containing multiple non-consecutive instructions, to be rewritten into alternative instructions. When multiple instructions are involved in an expression, it is necessary that all of the instructions have the necessary rewrite-based flag present on them, and the rewritten instructions will generally have the intersection of the flags present on the input instruction.

In the following example, the floating-point expression in the body of `@orig` has `contract` and `reassoc` in common, and thus if it is rewritten into the expression in the body of `@target`, all of the new instructions get those two flags and only those flags as a result. Since the `arcp` is present on only one of the instructions in the expression, it is not present in the transformed expression. Furthermore, this reassociation here is only legal because both the instructions had the `reassoc` flag; if only one had it, it would not be legal to make the transformation.

```
<span></span><span class="k">define</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="vg">@orig</span><span class="p">(</span><span class="kt">double</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%b</span><span class="p">,</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%c</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%t1</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">fmul</span><span class="w"> </span><span class="k">contract</span><span class="w"> </span><span class="k">reassoc</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="nv">%b</span>
<span class="w">  </span><span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">fmul</span><span class="w"> </span><span class="k">contract</span><span class="w"> </span><span class="k">reassoc</span><span class="w"> </span><span class="k">arcp</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%t1</span><span class="p">,</span><span class="w"> </span><span class="nv">%c</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%val</span>
<span class="p">}</span>

<span class="k">define</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="vg">@target</span><span class="p">(</span><span class="kt">double</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%b</span><span class="p">,</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%c</span><span class="p">)</span><span class="w"> </span><span class="p">{</span>
<span class="w">  </span><span class="nv">%t1</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">fmul</span><span class="w"> </span><span class="k">contract</span><span class="w"> </span><span class="k">reassoc</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%b</span><span class="p">,</span><span class="w"> </span><span class="nv">%c</span>
<span class="w">  </span><span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">fmul</span><span class="w"> </span><span class="k">contract</span><span class="w"> </span><span class="k">reassoc</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%a</span><span class="p">,</span><span class="w"> </span><span class="nv">%t1</span>
<span class="w">  </span><span class="k">ret</span><span class="w"> </span><span class="kt">double</span><span class="w"> </span><span class="nv">%val</span>
<span class="p">}</span>
```

These rules do not apply to the other fast-math flags. Whether or not a flag like `nnan` is present on any or all of the rewritten instructions is based on whether or not it is possible for said instruction to have a NaN input or output, given the original flags.

`arcp`

Allows division to be treated as a multiplication by a reciprocal. Specifically, this permits `a / b` to be considered equivalent to `a * (1.0 / b)` (which may subsequently be susceptible to code motion), and it also permits `a / (b / c)` to be considered equivalent to `a * (c / b)`. Both of these rewrites can be applied in either direction: `a * (c / b)` can be rewritten into `a / (b / c)`.

`contract`

Allow floating-point contraction (e.g. fusing a multiply followed by an addition into a fused multiply-and-add). This does not enable reassociation to form arbitrary contractions. For example, `(a*b) + (c*d) + e` can not be transformed into `(a*b) + ((c*d) + e)` to create two fma operations.

`afn`

Approximate functions - Allow substitution of approximate calculations for functions (sin, log, sqrt, etc). See floating-point intrinsic definitions for places where this can apply to LLVM’s intrinsic math functions.

`reassoc`

Allow algebraically equivalent transformations for floating-point instructions such as reassociation transformations. This may dramatically change results in floating-point.

### [Use-list Order Directives](https://llvm.org/docs/LangRef.html#id2023)[¶](https://llvm.org/docs/LangRef.html#use-list-order-directives "Link to this heading")

Use-list directives encode the in-memory order of each use-list, allowing the order to be recreated. `<order-indexes>` is a comma-separated list of indexes that are assigned to the referenced value’s uses. The referenced value’s use-list is immediately sorted by these indexes.

Use-list directives may appear at function scope or global scope. They are not instructions, and have no effect on the semantics of the IR. When they’re at function scope, they must appear after the terminator of the final basic block.

If basic blocks have their address taken via `blockaddress()` expressions, `uselistorder_bb` can be used to reorder their use-lists from outside their function’s scope.

Syntax:

```
<span></span><span class="n">uselistorder</span> <span class="o">&lt;</span><span class="n">ty</span><span class="o">&gt;</span> <span class="o">&lt;</span><span class="n">value</span><span class="o">&gt;</span><span class="p">,</span> <span class="p">{</span> <span class="o">&lt;</span><span class="n">order</span><span class="o">-</span><span class="n">indexes</span><span class="o">&gt;</span> <span class="p">}</span>
<span class="n">uselistorder_bb</span> <span class="nd">@function</span><span class="p">,</span> <span class="o">%</span><span class="n">block</span> <span class="p">{</span> <span class="o">&lt;</span><span class="n">order</span><span class="o">-</span><span class="n">indexes</span><span class="o">&gt;</span> <span class="p">}</span>
```

Examples:

```
<span></span><span class="n">define</span> <span class="n">void</span> <span class="nd">@foo</span><span class="p">(</span><span class="n">i32</span> <span class="o">%</span><span class="n">arg1</span><span class="p">,</span> <span class="n">i32</span> <span class="o">%</span><span class="n">arg2</span><span class="p">)</span> <span class="p">{</span>
<span class="n">entry</span><span class="p">:</span>
  <span class="p">;</span> <span class="o">...</span> <span class="n">instructions</span> <span class="o">...</span>
<span class="n">bb</span><span class="p">:</span>
  <span class="p">;</span> <span class="o">...</span> <span class="n">instructions</span> <span class="o">...</span>

  <span class="p">;</span> <span class="n">At</span> <span class="n">function</span> <span class="n">scope</span><span class="o">.</span>
  <span class="n">uselistorder</span> <span class="n">i32</span> <span class="o">%</span><span class="n">arg1</span><span class="p">,</span> <span class="p">{</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">0</span><span class="p">,</span> <span class="mi">2</span> <span class="p">}</span>
  <span class="n">uselistorder</span> <span class="n">label</span> <span class="o">%</span><span class="n">bb</span><span class="p">,</span> <span class="p">{</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">0</span> <span class="p">}</span>
<span class="p">}</span>

<span class="p">;</span> <span class="n">At</span> <span class="k">global</span> <span class="n">scope</span><span class="o">.</span>
<span class="n">uselistorder</span> <span class="n">ptr</span> <span class="nd">@global</span><span class="p">,</span> <span class="p">{</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">2</span><span class="p">,</span> <span class="mi">0</span> <span class="p">}</span>
<span class="n">uselistorder</span> <span class="n">i32</span> <span class="mi">7</span><span class="p">,</span> <span class="p">{</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">0</span> <span class="p">}</span>
<span class="n">uselistorder</span> <span class="n">i32</span> <span class="p">(</span><span class="n">i32</span><span class="p">)</span> <span class="nd">@bar</span><span class="p">,</span> <span class="p">{</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">0</span> <span class="p">}</span>
<span class="n">uselistorder_bb</span> <span class="nd">@foo</span><span class="p">,</span> <span class="o">%</span><span class="n">bb</span><span class="p">,</span> <span class="p">{</span> <span class="mi">5</span><span class="p">,</span> <span class="mi">1</span><span class="p">,</span> <span class="mi">3</span><span class="p">,</span> <span class="mi">2</span><span class="p">,</span> <span class="mi">0</span><span class="p">,</span> <span class="mi">4</span> <span class="p">}</span>
```

### [Source Filename](https://llvm.org/docs/LangRef.html#id2024)[¶](https://llvm.org/docs/LangRef.html#source-filename "Link to this heading")

The _source filename_ string is set to the original module identifier, which will be the name of the compiled source file when compiling from source through the clang front end, for example. It is then preserved through the IR and bitcode.

This is currently necessary to generate a consistent unique global identifier for local functions used in profile data, which prepends the source file name to the local function name.

The syntax for the source file name is simply:

```
<span></span>source_filename = "/path/to/source.c"
```

## [Type System](https://llvm.org/docs/LangRef.html#id2025)[¶](https://llvm.org/docs/LangRef.html#type-system "Link to this heading")

The LLVM type system is one of the most important features of the intermediate representation. Being typed enables a number of optimizations to be performed on the intermediate representation directly, without having to do extra analyses on the side before the transformation. A strong type system makes it easier to read the generated code and enables novel analyses and transformations that are not feasible to perform on normal three address code representations.

### [Void Type](https://llvm.org/docs/LangRef.html#id2026)[¶](https://llvm.org/docs/LangRef.html#void-type "Link to this heading")

Overview:

The void type does not represent any value and has no size.

Syntax:

```
<span></span><span class="n">void</span>
```

### [Function Type](https://llvm.org/docs/LangRef.html#id2027)[¶](https://llvm.org/docs/LangRef.html#function-type "Link to this heading")

Overview:

The function type can be thought of as a function signature. It consists of a return type and a list of formal parameter types. The return type of a function type is a void type or first class type — except for [label](https://llvm.org/docs/LangRef.html#t-label) and [metadata](https://llvm.org/docs/LangRef.html#t-metadata) types.

Syntax:

```
<span></span><span class="o">&lt;</span><span class="n">returntype</span><span class="o">&gt;</span> <span class="p">(</span><span class="o">&lt;</span><span class="n">parameter</span> <span class="nb">list</span><span class="o">&gt;</span><span class="p">)</span>
```

…where ‘`<parameter list>`’ is a comma-separated list of type specifiers. Optionally, the parameter list may include a type `...`, which indicates that the function takes a variable number of arguments. Variable argument functions can access their arguments with the [variable argument handling intrinsic](https://llvm.org/docs/LangRef.html#int-varargs) functions. ‘`<returntype>`’ is any type except [label](https://llvm.org/docs/LangRef.html#t-label) and [metadata](https://llvm.org/docs/LangRef.html#t-metadata).

Examples:

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">i32</span> <span class="pre">(i32)</span></code></p></td><td><p>function taking an <code class="docutils literal notranslate"><span class="pre">i32</span></code>, returning an <code class="docutils literal notranslate"><span class="pre">i32</span></code></p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">i32</span> <span class="pre">(ptr,</span> <span class="pre">...)</span></code></p></td><td><p>A vararg function that takes at least one <a class="reference internal" href="https://llvm.org/docs/LangRef.html#t-pointer"><span class="std std-ref">pointer</span></a> argument and returns an integer. This is the signature for <code class="docutils literal notranslate"><span class="pre">printf</span></code> in LLVM.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">{i32,</span> <span class="pre">i32}</span> <span class="pre">(i32)</span></code></p></td><td><p>A function taking an <code class="docutils literal notranslate"><span class="pre">i32</span></code>, returning a <a class="reference internal" href="https://llvm.org/docs/LangRef.html#t-struct"><span class="std std-ref">structure</span></a> containing two <code class="docutils literal notranslate"><span class="pre">i32</span></code> values</p></td></tr></tbody></table>

### [Opaque Structure Types](https://llvm.org/docs/LangRef.html#id2028)[¶](https://llvm.org/docs/LangRef.html#opaque-structure-types "Link to this heading")

Overview:

Opaque structure types are used to represent structure types that do not have a body specified. This corresponds (for example) to the C notion of a forward declared structure. They can be named (`%X`) or unnamed (`%52`).

It is not possible to create SSA values with an opaque structure type. In practice, this largely limits their use to the value type of external globals.

Syntax:

```
<span></span><span class="o">%</span><span class="n">X</span> <span class="o">=</span> <span class="nb">type</span> <span class="n">opaque</span>
<span class="o">%</span><span class="mi">52</span> <span class="o">=</span> <span class="nb">type</span> <span class="n">opaque</span>

<span class="nd">@g</span> <span class="o">=</span> <span class="n">external</span> <span class="k">global</span> <span class="o">%</span><span class="n">X</span>
```

### [First Class Types](https://llvm.org/docs/LangRef.html#id2029)[¶](https://llvm.org/docs/LangRef.html#first-class-types "Link to this heading")

The [first class](https://llvm.org/docs/LangRef.html#t-firstclass) types are perhaps the most important. Values of these types are the only ones which can be produced by instructions.

#### [Single Value Types](https://llvm.org/docs/LangRef.html#id2030)[¶](https://llvm.org/docs/LangRef.html#single-value-types "Link to this heading")

These are the types that are valid in registers from CodeGen’s perspective.

##### Integer Type[¶](https://llvm.org/docs/LangRef.html#integer-type "Link to this heading")

Overview:

The integer type is a very simple type that simply specifies an arbitrary bit width for the integer type desired. Any bit width from 1 bit to 2<sup>23</sup>(about 8 million) can be specified.

Syntax:

```
<span></span><span class="n">iN</span>
```

The number of bits the integer will occupy is specified by the `N` value.

###### Examples:[¶](https://llvm.org/docs/LangRef.html#examples "Link to this heading")

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">i1</span></code></p></td><td><p>a single-bit integer.</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">i32</span></code></p></td><td><p>a 32-bit integer.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">i1942652</span></code></p></td><td><p>a really big integer of over 1 million bits.</p></td></tr></tbody></table>

##### Floating-Point Types[¶](https://llvm.org/docs/LangRef.html#floating-point-types "Link to this heading")

| 
Type

 | 

Description

 |
| --- | --- |
| 

`half`

 | 

16-bit floating-point value (IEEE-754 binary16)

 |
| 

`bfloat`

 | 

16-bit “brain” floating-point value (7-bit significand). Provides the same number of exponent bits as `float`, so that it matches its dynamic range, but with greatly reduced precision. Used in Intel’s AVX-512 BF16 extensions and Arm’s ARMv8.6-A extensions, among others.

 |
| 

`float`

 | 

32-bit floating-point value (IEEE-754 binary32)

 |
| 

`double`

 | 

64-bit floating-point value (IEEE-754 binary64)

 |
| 

`fp128`

 | 

128-bit floating-point value (IEEE-754 binary128)

 |
| 

`x86_fp80`

 | 

80-bit floating-point value (X87)

 |
| 

`ppc_fp128`

 | 

128-bit floating-point value (two 64-bits)

 |

##### X86\_amx Type[¶](https://llvm.org/docs/LangRef.html#x86-amx-type "Link to this heading")

Overview:

The x86\_amx type represents a value held in an AMX tile register on an x86 machine. The operations allowed on it are quite limited. Only a few intrinsics are allowed: stride load and store, zero and dot product. No instruction is allowed for this type. There are no arguments, arrays, pointers, vectors or constants of this type.

Syntax:

```
<span></span><span class="n">x86_amx</span>
```

##### Pointer Type[¶](https://llvm.org/docs/LangRef.html#pointer-type "Link to this heading")

Overview:

The pointer type `ptr` is used to specify memory locations. Pointers are commonly used to reference objects in memory.

Pointer types may have an optional address space attribute defining the numbered address space where the pointed-to object resides. For example, `ptr addrspace(5)` is a pointer to address space 5. In addition to integer constants, `addrspace` can also reference one of the address spaces defined in the [datalayout string](https://llvm.org/docs/LangRef.html#langref-datalayout). `addrspace("A")` will use the alloca address space, `addrspace("G")` the default globals address space and `addrspace("P")` the program address space.

The representation of pointers can be different for each address space and does not necessarily need to be a plain integer address (e.g. for [non-integral pointers](https://llvm.org/docs/LangRef.html#nointptrtype)). In addition to a representation bits size, pointers in each address space also have an index size which defines the bitwidth of indexing operations as well as the size of integer addresses in this address space. For example, CHERI capabilities are twice the size of the underlying addresses to accommodate for additional metadata such as bounds and permissions: on a 32-bit system the bitwidth of the pointer representation size is 64, but the underlying address width remains 32 bits.

The default address space is number zero.

The semantics of non-zero address spaces are target-specific. Memory access through a non-dereferenceable pointer is undefined behavior in any address space. Pointers with the bit-value 0 are only assumed to be non-dereferenceable in address space 0, unless the function is marked with the `null_pointer_is_valid` attribute. However, _volatile_ access to any non-dereferenceable address may have defined behavior (according to the target), and in this case the attribute is not needed even for address 0.

If an object can be proven accessible through a pointer with a different address space, the access may be modified to use that address space. Exceptions apply if the operation is `volatile`.

Prior to LLVM 15, pointer types also specified a pointee type, such as `i8*`, `[4 x i32]*` or `i32 (i32*)*`. In LLVM 15, such “typed pointers” are still supported under non-default options. See the [opaque pointers document](https://llvm.org/docs/LangRef.htmlOpaquePointers.html) for more information.

##### Target Extension Type[¶](https://llvm.org/docs/LangRef.html#target-extension-type "Link to this heading")

Overview:

Target extension types represent types that must be preserved through optimization, but are otherwise generally opaque to the compiler. They may be used as function parameters or arguments, and in [phi](https://llvm.org/docs/LangRef.html#i-phi) or [select](https://llvm.org/docs/LangRef.html#i-select) instructions. Some types may be also used in [alloca](https://llvm.org/docs/LangRef.html#i-alloca) instructions or as global values, and correspondingly it is legal to use [load](https://llvm.org/docs/LangRef.html#i-load) and [store](https://llvm.org/docs/LangRef.html#i-store) instructions on them. Full semantics for these types are defined by the target.

The only constants that target extension types may have are `zeroinitializer`, `undef`, and `poison`. Other possible values for target extension types may arise from target-specific intrinsics and functions.

These types cannot be converted to other types. As such, it is not legal to use them in [bitcast](https://llvm.org/docs/LangRef.html#i-bitcast) instructions (as a source or target type), nor is it legal to use them in [ptrtoint](https://llvm.org/docs/LangRef.html#i-ptrtoint) or [inttoptr](https://llvm.org/docs/LangRef.html#i-inttoptr) instructions. Similarly, they are not legal to use in an [icmp](https://llvm.org/docs/LangRef.html#i-icmp) instruction.

Target extension types have a name and optional type or integer parameters. The meanings of name and parameters are defined by the target. When being defined in LLVM IR, all of the type parameters must precede all of the integer parameters.

Specific target extension types are registered with LLVM as having specific properties. These properties can be used to restrict the type from appearing in certain contexts, such as being the type of a global variable or having a `zeroinitializer` constant be valid. A complete list of type properties may be found in the documentation for `llvm::TargetExtType::Property` ([doxygen](https://llvm.org/doxygen/classllvm_1_1TargetExtType.html)).

Syntax:

```
<span></span><span class="k">target</span><span class="p">(</span><span class="s">"label"</span><span class="p">)</span>
<span class="k">target</span><span class="p">(</span><span class="s">"label"</span><span class="p">,</span><span class="w"> </span><span class="k">void</span><span class="p">)</span>
<span class="k">target</span><span class="p">(</span><span class="s">"label"</span><span class="p">,</span><span class="w"> </span><span class="k">void</span><span class="p">,</span><span class="w"> </span><span class="kt">i32</span><span class="p">)</span>
<span class="k">target</span><span class="p">(</span><span class="s">"label"</span><span class="p">,</span><span class="w"> </span><span class="m">0</span><span class="p">,</span><span class="w"> </span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="m">2</span><span class="p">)</span>
<span class="k">target</span><span class="p">(</span><span class="s">"label"</span><span class="p">,</span><span class="w"> </span><span class="k">void</span><span class="p">,</span><span class="w"> </span><span class="kt">i32</span><span class="p">,</span><span class="w"> </span><span class="m">0</span><span class="p">,</span><span class="w"> </span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="m">2</span><span class="p">)</span>
```

##### Vector Type[¶](https://llvm.org/docs/LangRef.html#vector-type "Link to this heading")

Overview:

A vector type is a simple derived type that represents a vector of elements. Vector types are used when multiple primitive data are operated in parallel using a single instruction (SIMD). A vector type requires a size (number of elements), an underlying primitive data type, and a scalable property to represent vectors where the exact hardware vector length is unknown at compile time. Vector types are considered [first class](https://llvm.org/docs/LangRef.html#t-firstclass).

Memory Layout:

In general vector elements are laid out in memory in the same way as [array types](https://llvm.org/docs/LangRef.html#t-array). Such an analogy works fine as long as the vector elements are byte sized. However, when the elements of the vector aren’t byte sized it gets a bit more complicated. One way to describe the layout is by describing what happens when a vector such as <N x iM> is bitcasted to an integer type with N\*M bits, and then following the rules for storing such an integer to memory.

A bitcast from a vector type to a scalar integer type will see the elements being packed together (without padding). The order in which elements are inserted in the integer depends on endianness. For little endian element zero is put in the least significant bits of the integer, and for big endian element zero is put in the most significant bits.

Using a vector such as `<i4 1, i4 2, i4 3, i4 5>` as an example, together with the analogy that we can replace a vector store by a bitcast followed by an integer store, we get this for big endian:

```
<span></span><span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">bitcast</span><span class="w"> </span><span class="p">&lt;</span><span class="m">4</span><span class="w"> </span><span class="k">x</span><span class="w"> </span><span class="kt">i4</span><span class="p">&gt;</span><span class="w"> </span><span class="p">&lt;</span><span class="kt">i4</span><span class="w"> </span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">2</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">3</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">5</span><span class="p">&gt;</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i16</span>

<span class="c">; Bitcasting from a vector to an integral type can be seen as</span>
<span class="c">; concatenating the values:</span>
<span class="c">;   %val now has the hexadecimal value 0x1235.</span>

<span class="k">store</span><span class="w"> </span><span class="kt">i16</span><span class="w"> </span><span class="nv">%val</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%ptr</span>

<span class="c">; In memory the content will be (8-bit addressing):</span>
<span class="c">;</span>
<span class="c">;    [%ptr + 0]: 00010010  (0x12)</span>
<span class="c">;    [%ptr + 1]: 00110101  (0x35)</span>
```

The same example for little endian:

```
<span></span><span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">bitcast</span><span class="w"> </span><span class="p">&lt;</span><span class="m">4</span><span class="w"> </span><span class="k">x</span><span class="w"> </span><span class="kt">i4</span><span class="p">&gt;</span><span class="w"> </span><span class="p">&lt;</span><span class="kt">i4</span><span class="w"> </span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">2</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">3</span><span class="p">,</span><span class="w"> </span><span class="kt">i4</span><span class="w"> </span><span class="m">5</span><span class="p">&gt;</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i16</span>

<span class="c">; Bitcasting from a vector to an integral type can be seen as</span>
<span class="c">; concatenating the values:</span>
<span class="c">;   %val now has the hexadecimal value 0x5321.</span>

<span class="k">store</span><span class="w"> </span><span class="kt">i16</span><span class="w"> </span><span class="nv">%val</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%ptr</span>

<span class="c">; In memory the content will be (8-bit addressing):</span>
<span class="c">;</span>
<span class="c">;    [%ptr + 0]: 00100001  (0x21)</span>
<span class="c">;    [%ptr + 1]: 01010011  (0x53)</span>
```

When `<N*M>` isn’t evenly divisible by the byte size the exact memory layout is unspecified (just like it is for an integral type of the same size). This is because different targets could put the padding at different positions when the type size is smaller than the type’s store size.

Syntax:

```
<span></span><span class="o">&lt;</span> <span class="o">&lt;</span><span class="c1"># elements&gt; x &lt;elementtype&gt; &gt;          ; Fixed-length vector</span>
<span class="o">&lt;</span> <span class="n">vscale</span> <span class="n">x</span> <span class="o">&lt;</span><span class="c1"># elements&gt; x &lt;elementtype&gt; &gt; ; Scalable vector</span>
```

The number of elements is a constant integer value larger than 0; elementtype may be any integer, floating-point, pointer type, or a sized target extension type that has the `CanBeVectorElement` property. Vectors of size zero are not allowed. For scalable vectors, the total number of elements is a constant multiple (called vscale) of the specified number of elements; vscale is a positive integer that is unknown at compile time and the same hardware-dependent constant for all scalable vectors at run time. The size of a specific scalable vector type is thus constant within IR, even if the exact size in bytes cannot be determined until run time.

Examples:

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;4</span> <span class="pre">x</span> <span class="pre">i32&gt;</span></code></p></td><td><p>Vector of 4 32-bit integer values.</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;8</span> <span class="pre">x</span> <span class="pre">float&gt;</span></code></p></td><td><p>Vector of 8 32-bit floating-point values.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;2</span> <span class="pre">x</span> <span class="pre">i64&gt;</span></code></p></td><td><p>Vector of 2 64-bit integer values.</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;4</span> <span class="pre">x</span> <span class="pre">ptr&gt;</span></code></p></td><td><p>Vector of 4 pointers</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;vscale</span> <span class="pre">x</span> <span class="pre">4</span> <span class="pre">x</span> <span class="pre">i32&gt;</span></code></p></td><td><p>Vector with a multiple of 4 32-bit integer values.</p></td></tr></tbody></table>

#### [Label Type](https://llvm.org/docs/LangRef.html#id2031)[¶](https://llvm.org/docs/LangRef.html#label-type "Link to this heading")

Overview:

The label type represents code labels.

Syntax:

```
<span></span><span class="n">label</span>
```

#### [Token Type](https://llvm.org/docs/LangRef.html#id2032)[¶](https://llvm.org/docs/LangRef.html#token-type "Link to this heading")

Overview:

The token type is used when a value is associated with an instruction but all uses of the value must not attempt to introspect or obscure it. As such, it is not appropriate to have a [phi](https://llvm.org/docs/LangRef.html#i-phi) or [select](https://llvm.org/docs/LangRef.html#i-select) of type token.

Syntax:

```
<span></span><span class="n">token</span>
```

#### [Metadata Type](https://llvm.org/docs/LangRef.html#id2033)[¶](https://llvm.org/docs/LangRef.html#metadata-type "Link to this heading")

Overview:

The metadata type represents embedded metadata. No derived types may be created from metadata except for [function](https://llvm.org/docs/LangRef.html#t-function) arguments.

Syntax:

```
<span></span><span class="n">metadata</span>
```

#### [Aggregate Types](https://llvm.org/docs/LangRef.html#id2034)[¶](https://llvm.org/docs/LangRef.html#aggregate-types "Link to this heading")

Aggregate Types are a subset of derived types that can contain multiple member types. [Arrays](https://llvm.org/docs/LangRef.html#t-array) and [structs](https://llvm.org/docs/LangRef.html#t-struct) are aggregate types. [Vectors](https://llvm.org/docs/LangRef.html#t-vector) are not considered to be aggregate types.

##### Array Type[¶](https://llvm.org/docs/LangRef.html#array-type "Link to this heading")

Overview:

The array type is a very simple derived type that arranges elements sequentially in memory. The array type requires a size (number of elements) and an underlying data type.

Syntax:

```
<span></span><span class="p">[</span><span class="o">&lt;</span><span class="c1"># elements&gt; x &lt;elementtype&gt;]</span>
```

The number of elements is a constant integer value; `elementtype` may be any type with a size.

Examples:

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">[40</span> <span class="pre">x</span> <span class="pre">i32]</span></code></p></td><td><p>Array of 40 32-bit integer values.</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">[41</span> <span class="pre">x</span> <span class="pre">i32]</span></code></p></td><td><p>Array of 41 32-bit integer values.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">[4</span> <span class="pre">x</span> <span class="pre">i8]</span></code></p></td><td><p>Array of 4 8-bit integer values.</p></td></tr></tbody></table>

Here are some examples of multidimensional arrays:

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">[3</span> <span class="pre">x</span> <span class="pre">[4</span> <span class="pre">x</span> <span class="pre">i32]]</span></code></p></td><td><p>3x4 array of 32-bit integer values.</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">[12</span> <span class="pre">x</span> <span class="pre">[10</span> <span class="pre">x</span> <span class="pre">float]]</span></code></p></td><td><p>12x10 array of single precision floating-point values.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">[2</span> <span class="pre">x</span> <span class="pre">[3</span> <span class="pre">x</span> <span class="pre">[4</span> <span class="pre">x</span> <span class="pre">i16]]]</span></code></p></td><td><p>2x3x4 array of 16-bit integer values.</p></td></tr></tbody></table>

There is no restriction on indexing beyond the end of the array implied by a static type (though there are restrictions on indexing beyond the bounds of an [allocated object](https://llvm.org/docs/LangRef.html#allocatedobjects) in some cases). This means that single-dimension ‘variable sized array’ addressing can be implemented in LLVM with a zero length array type. An implementation of ‘pascal style arrays’ in LLVM could use the type “`{ i32, [0 x float]}`”, for example.

##### Structure Type[¶](https://llvm.org/docs/LangRef.html#structure-type "Link to this heading")

Overview:

The structure type is used to represent a collection of data members together in memory. The elements of a structure may be any type that has a size.

Structures in memory are accessed using ‘`load`’ and ‘`store`’ by getting a pointer to a field with the ‘`getelementptr`’ instruction. Structures in registers are accessed using the ‘`extractvalue`’ and ‘`insertvalue`’ instructions.

Structures may optionally be “packed” structures, which indicate that the alignment of the struct is one byte, and that there is no padding between the elements. In non-packed structs, padding between field types is inserted as defined by the DataLayout string in the module, which is required to match what the underlying code generator expects.

Structures can either be “literal” or “identified”. A literal structure is defined inline with other types (e.g. `[2 x {i32, i32}]`) whereas identified types are always defined at the top level with a name. Literal types are uniqued by their contents and can never be recursive or opaque since there is no way to write one. Identified types can be opaqued and are never uniqued. Identified types must not be recursive.

Syntax:

```
<span></span><span class="o">%</span><span class="n">T1</span> <span class="o">=</span> <span class="nb">type</span> <span class="p">{</span> <span class="o">&lt;</span><span class="nb">type</span> <span class="nb">list</span><span class="o">&gt;</span> <span class="p">}</span>     <span class="p">;</span> <span class="n">Identified</span> <span class="n">normal</span> <span class="n">struct</span> <span class="nb">type</span>
<span class="o">%</span><span class="n">T2</span> <span class="o">=</span> <span class="nb">type</span> <span class="o">&lt;</span><span class="p">{</span> <span class="o">&lt;</span><span class="nb">type</span> <span class="nb">list</span><span class="o">&gt;</span> <span class="p">}</span><span class="o">&gt;</span>   <span class="p">;</span> <span class="n">Identified</span> <span class="n">packed</span> <span class="n">struct</span> <span class="nb">type</span>
```

Examples:

<table class="docutils align-default"><tbody><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">{</span> <span class="pre">i32,</span> <span class="pre">i32,</span> <span class="pre">i32</span> <span class="pre">}</span></code></p></td><td><p>A triple of three <code class="docutils literal notranslate"><span class="pre">i32</span></code> values (this is a “homogeneous” struct as all element types are the same)</p></td></tr><tr class="row-even"><td><p><code class="docutils literal notranslate"><span class="pre">{</span> <span class="pre">float,</span> <span class="pre">ptr</span> <span class="pre">}</span></code></p></td><td><p>A pair, where the first element is a <code class="docutils literal notranslate"><span class="pre">float</span></code> and the second element is a <a class="reference internal" href="https://llvm.org/docs/LangRef.html#t-pointer"><span class="std std-ref">pointer</span></a>.</p></td></tr><tr class="row-odd"><td><p><code class="docutils literal notranslate"><span class="pre">&lt;{</span> <span class="pre">i8,</span> <span class="pre">i32</span> <span class="pre">}&gt;</span></code></p></td><td><p>A packed struct known to be 5 bytes in size.</p></td></tr></tbody></table>

## [Constants](https://llvm.org/docs/LangRef.html#id2035)[¶](https://llvm.org/docs/LangRef.html#constants "Link to this heading")

LLVM has several different basic types of constants. This section describes them all and their syntax.

### [Simple Constants](https://llvm.org/docs/LangRef.html#id2036)[¶](https://llvm.org/docs/LangRef.html#simple-constants "Link to this heading")

**Boolean constants**

The two strings ‘`true`’ and ‘`false`’ are both valid constants of the `i1` type.

**Integer constants**

Standard integers (such as ‘4’) are constants of the [integer](https://llvm.org/docs/LangRef.html#t-integer) type. They can be either decimal or hexadecimal. Decimal integers can be prefixed with - to represent negative integers, e.g. ‘`-1234`’. Hexadecimal integers must be prefixed with either u or s to indicate whether they are unsigned or signed respectively. e.g ‘`u0x8000`’ gives 32768, whilst ‘`s0x8000`’ gives -32768.

Note that hexadecimal integers are sign extended from the number of active bits, i.e., the bit width minus the number of leading zeros. So ‘`s0x0001`’ of type ‘`i16`’ will be -1, not 1.

**Floating-point constants**

Floating-point constants use standard decimal notation (e.g. 123.421), exponential notation (e.g. 1.23421e+2), or a more precise hexadecimal notation (see below). The assembler requires the exact decimal value of a floating-point constant. For example, the assembler accepts 1.25 but rejects 1.3 because 1.3 is a repeating decimal in binary. Floating-point constants must have a [floating-point](https://llvm.org/docs/LangRef.html#t-floating) type.

**Null pointer constants**

The identifier ‘`null`’ is recognized as a null pointer constant and must be of [pointer type](https://llvm.org/docs/LangRef.html#t-pointer).

**Token constants**

The identifier ‘`none`’ is recognized as an empty token constant and must be of [token type](https://llvm.org/docs/LangRef.html#t-token).

The one non-intuitive notation for constants is the hexadecimal form of floating-point constants. For example, the form ‘`double    0x432ff973cafa8000`’ is equivalent to (but harder to read than) ‘`double 4.5e+15`’. The only time hexadecimal floating-point constants are required (and the only time that they are generated by the disassembler) is when a floating-point constant must be emitted but it cannot be represented as a decimal floating-point number in a reasonable number of digits. For example, NaN’s, infinities, and other special values are represented in their IEEE hexadecimal format so that assembly and disassembly do not cause any bits to change in the constants.

When using the hexadecimal form, constants of types bfloat, half, float, and double are represented using the 16-digit form shown above (which matches the IEEE754 representation for double); bfloat, half and float values must, however, be exactly representable as bfloat, IEEE 754 half, and IEEE 754 single precision respectively. Hexadecimal format is always used for long double, and there are three forms of long double. The 80-bit format used by x86 is represented as `0xK` followed by 20 hexadecimal digits. The 128-bit format used by PowerPC (two adjacent doubles) is represented by `0xM` followed by 32 hexadecimal digits. The IEEE 128-bit format is represented by `0xL` followed by 32 hexadecimal digits. Long doubles will only work if they match the long double format on your target. The IEEE 16-bit format (half precision) is represented by `0xH` followed by 4 hexadecimal digits. The bfloat 16-bit format is represented by `0xR` followed by 4 hexadecimal digits. All hexadecimal formats are big-endian (sign bit at the left).

There are no constants of type x86\_amx.

### [Complex Constants](https://llvm.org/docs/LangRef.html#id2037)[¶](https://llvm.org/docs/LangRef.html#complex-constants "Link to this heading")

Complex constants are a (potentially recursive) combination of simple constants and smaller complex constants.

**Structure constants**

Structure constants are represented with notation similar to structure type definitions (a comma separated list of elements, surrounded by braces (`{}`)). For example: “`{ i32 4, float 17.0, ptr @G }`”, where “`@G`” is declared as “`@G = external global i32`”. Structure constants must have [structure type](https://llvm.org/docs/LangRef.html#t-struct), and the number and types of elements must match those specified by the type.

**Array constants**

Array constants are represented with notation similar to array type definitions (a comma separated list of elements, surrounded by square brackets (`[]`)). For example: “`[ i32 42, i32 11, i32 74 ]`”. Array constants must have [array type](https://llvm.org/docs/LangRef.html#t-array), and the number and types of elements must match those specified by the type. As a special case, character array constants may also be represented as a double-quoted string using the `c` prefix. For example: “`c"Hello World\0A\00"`”.

**Vector constants**

Vector constants are represented with notation similar to vector type definitions (a comma separated list of elements, surrounded by less-than/greater-than’s (`<>`)). For example: “`< i32 42, i32 11, i32 74, i32 100 >`”. Vector constants must have [vector type](https://llvm.org/docs/LangRef.html#t-vector), and the number and types of elements must match those specified by the type.

When creating a vector whose elements have the same constant value, the preferred syntax is `splat (<Ty> Val)`. For example: “`splat (i32 11)`”. These vector constants must have [vector type](https://llvm.org/docs/LangRef.html#t-vector) with an element type that matches the `splat` operand.

**Zero initialization**

The string ‘`zeroinitializer`’ can be used to zero initialize a value to zero of _any_ type, including scalar and [aggregate](https://llvm.org/docs/LangRef.html#t-aggregate) types. This is often used to avoid having to print large zero initializers (e.g. for large arrays) and is always exactly equivalent to using explicit zero initializers.

**Metadata node**

A metadata node is a constant tuple without types. For example: “`!{!0, !{!2, !0}, !"test"}`”. Metadata can reference constant values, for example: “`!{!0, i32 0, ptr @global, ptr @function, !"str"}`”. Unlike other typed constants that are meant to be interpreted as part of the instruction stream, metadata is a place to attach additional information such as debug info.

### [Global Variable and Function Addresses](https://llvm.org/docs/LangRef.html#id2038)[¶](https://llvm.org/docs/LangRef.html#global-variable-and-function-addresses "Link to this heading")

The addresses of [global variables](https://llvm.org/docs/LangRef.html#globalvars) and [functions](https://llvm.org/docs/LangRef.html#functionstructure) are always implicitly valid (link-time) constants. These constants are explicitly referenced when the [identifier for the global](https://llvm.org/docs/LangRef.html#identifiers) is used and always have [pointer](https://llvm.org/docs/LangRef.html#t-pointer) type. For example, the following is a legal LLVM file:

```
<span></span><span class="vg">@X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="m">17</span>
<span class="vg">@Y</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="m">42</span>
<span class="vg">@Z</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">global</span><span class="w"> </span><span class="p">[</span><span class="m">2</span><span class="w"> </span><span class="k">x</span><span class="w"> </span><span class="kt">ptr</span><span class="p">]</span><span class="w"> </span><span class="p">[</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@X</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@Y</span><span class="w"> </span><span class="p">]</span>
```

### [Undefined Values](https://llvm.org/docs/LangRef.html#id2039)[¶](https://llvm.org/docs/LangRef.html#undefined-values "Link to this heading")

The string ‘`undef`’ can be used anywhere a constant is expected, and indicates that the user of the value may receive an unspecified bit-pattern. Undefined values may be of any type (other than ‘`label`’ or ‘`void`’) and be used anywhere a constant is permitted.

Note

A ‘`poison`’ value (described in the next section) should be used instead of ‘`undef`’ whenever possible. Poison values are stronger than undef, and enable more optimizations. Just the existence of ‘`undef`’ blocks certain optimizations (see the examples below).

Undefined values are useful because they indicate to the compiler that the program is well defined no matter what value is used. This gives the compiler more freedom to optimize. Here are some examples of (potentially surprising) transformations that are valid (in pseudo IR):

```
<span></span><span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">add</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">sub</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">xor</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
```

This is safe because all of the output bits are affected by the undef bits. Any output bit can have a zero or one depending on the input bits.

```
<span></span><span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">or</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">and</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="m">-1</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="m">0</span>
<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="nv">%X</span><span class="w">  </span><span class="c">;; By choosing undef as 0</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="nv">%X</span><span class="w">  </span><span class="c">;; By choosing undef as -1</span>
<span class="nl">Unsafe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
```

These logical operations have bits that are not always affected by the input. For example, if `%X` has a zero bit, then the output of the ‘`and`’ operation will always be a zero for that bit, no matter what the corresponding bit from the ‘`undef`’ is. As such, it is unsafe to optimize or assume that the result of the ‘`and`’ is ‘`undef`’. However, it is safe to assume that all bits of the ‘`undef`’ could be 0, and optimize the ‘`and`’ to 0. Likewise, it is safe to assume that all the bits of the ‘`undef`’ operand to the ‘`or`’ could be set, allowing the ‘`or`’ to be folded to -1.

```
<span></span><span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">select</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="nv">%Y</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">select</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="m">42</span><span class="p">,</span><span class="w"> </span><span class="nv">%Y</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">select</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="nv">%Y</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="nv">%X</span><span class="w">     </span><span class="p">(</span><span class="k">or</span><span class="w"> </span><span class="nv">%Y</span><span class="p">)</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="m">42</span><span class="w">     </span><span class="p">(</span><span class="k">or</span><span class="w"> </span><span class="nv">%Y</span><span class="p">)</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="nv">%Y</span><span class="w">     </span><span class="p">(</span><span class="err">if</span><span class="w"> </span><span class="nv">%Y</span><span class="w"> </span><span class="err">is</span><span class="w"> </span><span class="err">provably</span><span class="w"> </span><span class="err">not</span><span class="w"> </span><span class="k">poison</span><span class="c">; unsafe otherwise)</span>
<span class="nl">Unsafe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
```

This set of examples shows that undefined ‘`select`’ conditions can go _either way_, but they have to come from one of the two operands. In the `%A` example, if `%X` and `%Y` were both known to have a clear low bit, then `%A` would have to have a cleared low bit. However, in the `%C` example, the optimizer is allowed to assume that the ‘`undef`’ operand could be the same as `%Y` if `%Y` is provably not ‘`poison`’, allowing the whole ‘`select`’ to be eliminated. This is because ‘`poison`’ is stronger than ‘`undef`’.

```
<span></span><span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">xor</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>

<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">xor</span><span class="w"> </span><span class="nv">%B</span><span class="p">,</span><span class="w"> </span><span class="nv">%B</span>

<span class="w">  </span><span class="nv">%D</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%E</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">icmp</span><span class="w"> </span><span class="k">slt</span><span class="w"> </span><span class="nv">%D</span><span class="p">,</span><span class="w"> </span><span class="m">4</span>
<span class="w">  </span><span class="nv">%F</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">icmp</span><span class="w"> </span><span class="k">sge</span><span class="w"> </span><span class="nv">%D</span><span class="p">,</span><span class="w"> </span><span class="m">4</span>

<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%C</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%D</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%E</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="nv">%F</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">undef</span>
```

This example points out that two ‘`undef`’ operands are not necessarily the same. This can be surprising to people (and also matches C semantics) where they assume that “`X^X`” is always zero, even if `X` is undefined. This isn’t true for a number of reasons, but the short answer is that an ‘`undef`’ “variable” can arbitrarily change its value over its “live range”. This is true because the variable doesn’t actually _have a live range_. Instead, the value is logically read from arbitrary registers that happen to be around when needed, so the value is not necessarily consistent over time. In fact, `%A` and `%C` need to have the same semantics or the core LLVM “replace all uses with” concept would not hold.

To ensure all uses of a given register observe the same value (even if ‘`undef`’), the [freeze instruction](https://llvm.org/docs/LangRef.html#i-freeze) can be used.

```
<span></span><span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">sdiv</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="nv">%X</span>
<span class="w">  </span><span class="nv">%B</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">sdiv</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="k">undef</span>
<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%A</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="m">0</span>
<span class="nl">b:</span><span class="w"> </span><span class="k">unreachable</span>
```

These examples show the crucial difference between an _undefined value_ and _undefined behavior_. An undefined value (like ‘`undef`’) is allowed to have an arbitrary bit-pattern. This means that the `%A` operation can be constant folded to ‘`0`’, because the ‘`undef`’ could be zero, and zero divided by any value is zero. However, in the second example, we can make a more aggressive assumption: because the `undef` is allowed to be an arbitrary value, we are allowed to assume that it could be zero. Since a divide by zero has _undefined behavior_, we are allowed to assume that the operation does not execute at all. This allows us to delete the divide and all code after it. Because the undefined operation “can’t happen”, the optimizer can assume that it occurs in dead code.

```
<span></span>a:  store undef -&gt; %X
b:  store %X -&gt; undef
Safe:
a: &lt;deleted&gt;     (if the stored value in %X is provably not poison)
b: unreachable
```

A store _of_ an undefined value can be assumed to not have any effect; we can assume that the value is overwritten with bits that happen to match what was already there. This argument is only valid if the stored value is provably not `poison`. However, a store _to_ an undefined location could clobber arbitrary memory, therefore, it has undefined behavior.

Branching on an undefined value is undefined behavior. This explains optimizations that depend on branch conditions to construct predicates, such as Correlated Value Propagation and Global Value Numbering. In case of switch instruction, the branch condition should be frozen, otherwise it is undefined behavior.

```
<span></span><span class="nl">Unsafe:</span>
<span class="w">  </span><span class="k">br</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="err">BB</span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="err">BB</span><span class="m">2</span><span class="w"> </span><span class="c">; UB</span>

<span class="w">  </span><span class="nv">%X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">and</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="m">255</span>
<span class="w">  </span><span class="k">switch</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%ret</span><span class="w"> </span><span class="p">[</span><span class="w"> </span><span class="p">..</span><span class="w"> </span><span class="p">]</span><span class="w"> </span><span class="c">; UB</span>

<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%ptr</span>
<span class="w">  </span><span class="nv">%X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">load</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%ptr</span><span class="w"> </span><span class="c">; %X is undef</span>
<span class="w">  </span><span class="k">switch</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%ret</span><span class="w"> </span><span class="p">[</span><span class="w"> </span><span class="p">..</span><span class="w"> </span><span class="p">]</span><span class="w"> </span><span class="c">; UB</span>

<span class="nl">Safe:</span>
<span class="w">  </span><span class="nv">%X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">or</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="k">undef</span><span class="p">,</span><span class="w"> </span><span class="m">255</span><span class="w"> </span><span class="c">; always 255</span>
<span class="w">  </span><span class="k">switch</span><span class="w"> </span><span class="kt">i8</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%ret</span><span class="w"> </span><span class="p">[</span><span class="w"> </span><span class="p">..</span><span class="w"> </span><span class="p">]</span><span class="w"> </span><span class="c">; Well-defined</span>

<span class="w">  </span><span class="nv">%X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">freeze</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="k">undef</span>
<span class="w">  </span><span class="k">br</span><span class="w"> </span><span class="nv">%X</span><span class="p">,</span><span class="w"> </span><span class="err">BB</span><span class="m">1</span><span class="p">,</span><span class="w"> </span><span class="err">BB</span><span class="m">2</span><span class="w"> </span><span class="c">; Well-defined (non-deterministic jump)</span>
```

### [Poison Values](https://llvm.org/docs/LangRef.html#id2040)[¶](https://llvm.org/docs/LangRef.html#poison-values "Link to this heading")

A poison value is a result of an erroneous operation. In order to facilitate speculative execution, many instructions do not invoke immediate undefined behavior when provided with illegal operands, and return a poison value instead. The string ‘`poison`’ can be used anywhere a constant is expected, and operations such as [add](https://llvm.org/docs/LangRef.html#i-add) with the `nsw` flag can produce a poison value.

Most instructions return ‘`poison`’ when one of their arguments is ‘`poison`’. A notable exception is the [select instruction](https://llvm.org/docs/LangRef.html#i-select). Propagation of poison can be stopped with the [freeze instruction](https://llvm.org/docs/LangRef.html#i-freeze).

It is correct to replace a poison value with an [undef value](https://llvm.org/docs/LangRef.html#undefvalues) or any value of the type.

This means that immediate undefined behavior occurs if a poison value is used as an instruction operand that has any values that trigger undefined behavior. Notably this includes (but is not limited to):

-   The pointer operand of a [load](https://llvm.org/docs/LangRef.html#i-load), [store](https://llvm.org/docs/LangRef.html#i-store) or any other pointer dereferencing instruction (independent of address space).
    
-   The divisor operand of a `udiv`, `sdiv`, `urem` or `srem` instruction.
    
-   The condition operand of a [br](https://llvm.org/docs/LangRef.html#i-br) instruction.
    
-   The callee operand of a [call](https://llvm.org/docs/LangRef.html#i-call) or [invoke](https://llvm.org/docs/LangRef.html#i-invoke) instruction.
    
-   The parameter operand of a [call](https://llvm.org/docs/LangRef.html#i-call) or [invoke](https://llvm.org/docs/LangRef.html#i-invoke) instruction, when the function or invoking call site has a `noundef` attribute in the corresponding position.
    
-   The operand of a [ret](https://llvm.org/docs/LangRef.html#i-ret) instruction if the function or invoking call site has a noundef attribute in the return value position.
    

Here are some examples:

```
<span></span><span class="nl">entry:</span>
<span class="w">  </span><span class="nv">%poison</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">sub</span><span class="w"> </span><span class="k">nuw</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="m">0</span><span class="p">,</span><span class="w"> </span><span class="m">1</span><span class="w">           </span><span class="c">; Results in a poison value.</span>
<span class="w">  </span><span class="nv">%poison2</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">sub</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="k">poison</span><span class="p">,</span><span class="w"> </span><span class="m">1</span><span class="w">         </span><span class="c">; Also results in a poison value.</span>
<span class="w">  </span><span class="nv">%still_poison</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">and</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="nv">%poison</span><span class="p">,</span><span class="w"> </span><span class="m">0</span><span class="w">   </span><span class="c">; 0, but also poison.</span>
<span class="w">  </span><span class="nv">%poison_yet_again</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">getelementptr</span><span class="w"> </span><span class="kt">i32</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@h</span><span class="p">,</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="nv">%still_poison</span>
<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="m">0</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="nv">%poison_yet_again</span><span class="w">   </span><span class="c">; Undefined behavior due to</span>
<span class="w">                                       </span><span class="c">; store to poison.</span>

<span class="w">  </span><span class="k">store</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="nv">%poison</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@g</span><span class="w">            </span><span class="c">; Poison value stored to memory.</span>
<span class="w">  </span><span class="nv">%poison3</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">load</span><span class="w"> </span><span class="kt">i32</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@g</span><span class="w">          </span><span class="c">; Poison value loaded back from memory.</span>

<span class="w">  </span><span class="nv">%poison4</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">load</span><span class="w"> </span><span class="kt">i16</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@g</span><span class="w">          </span><span class="c">; Returns a poison value.</span>
<span class="w">  </span><span class="nv">%poison5</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">load</span><span class="w"> </span><span class="kt">i64</span><span class="p">,</span><span class="w"> </span><span class="kt">ptr</span><span class="w"> </span><span class="vg">@g</span><span class="w">          </span><span class="c">; Returns a poison value.</span>

<span class="w">  </span><span class="nv">%cmp</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">icmp</span><span class="w"> </span><span class="k">slt</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="nv">%poison</span><span class="p">,</span><span class="w"> </span><span class="m">0</span><span class="w">       </span><span class="c">; Returns a poison value.</span>
<span class="w">  </span><span class="k">br</span><span class="w"> </span><span class="kt">i1</span><span class="w"> </span><span class="nv">%cmp</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%end</span><span class="p">,</span><span class="w"> </span><span class="kt">label</span><span class="w"> </span><span class="nv">%end</span><span class="w">   </span><span class="c">; undefined behavior</span>

<span class="nl">end:</span>
```

### [Well-Defined Values](https://llvm.org/docs/LangRef.html#id2041)[¶](https://llvm.org/docs/LangRef.html#well-defined-values "Link to this heading")

Given a program execution, a value is _well defined_ if the value does not have an undef bit and is not poison in the execution. An aggregate value or vector is well defined if its elements are well defined. The padding of an aggregate isn’t considered, since it isn’t visible without storing it into memory and loading it with a different type.

A constant of a [single value](https://llvm.org/docs/LangRef.html#t-single-value), non-vector type is well defined if it is neither ‘`undef`’ constant nor ‘`poison`’ constant. The result of [freeze instruction](https://llvm.org/docs/LangRef.html#i-freeze) is well defined regardless of its operand.

### [Addresses of Basic Blocks](https://llvm.org/docs/LangRef.html#id2042)[¶](https://llvm.org/docs/LangRef.html#addresses-of-basic-blocks "Link to this heading")

`blockaddress(@function, %block)`

The ‘`blockaddress`’ constant computes the address of the specified basic block in the specified function.

It always has a `ptr addrspace(P)` type, where `P` is the address space of the function containing `%block` (usually `addrspace(0)`).

Taking the address of the entry block is illegal.

This value only has defined behavior when used as an operand to the ‘[indirectbr](https://llvm.org/docs/LangRef.html#i-indirectbr)’ or for comparisons against null. Pointer equality tests between label addresses results in undefined behavior — though, again, comparison against null is ok, and no label is equal to the null pointer. This may be passed around as an opaque pointer sized value as long as the bits are not inspected. This allows `ptrtoint` and arithmetic to be performed on these values so long as the original value is reconstituted before the `indirectbr` instruction.

Finally, some targets may provide defined semantics when using the value as the operand to an inline assembly, but that is target specific.

### [DSO Local Equivalent](https://llvm.org/docs/LangRef.html#id2043)[¶](https://llvm.org/docs/LangRef.html#dso-local-equivalent "Link to this heading")

`dso_local_equivalent @func`

A ‘`dso_local_equivalent`’ constant represents a function which is functionally equivalent to a given function, but is always defined in the current linkage unit. The resulting pointer has the same type as the underlying function. The resulting pointer is permitted, but not required, to be different from a pointer to the function, and it may have different values in different translation units.

The target function may not have `extern_weak` linkage.

`dso_local_equivalent` can be implemented as such:

-   If the function has local linkage, hidden visibility, or is `dso_local`, `dso_local_equivalent` can be implemented as simply a pointer to the function.
    
-   `dso_local_equivalent` can be implemented with a stub that tail-calls the function. Many targets support relocations that resolve at link time to either a function or a stub for it, depending on whether the function is defined within the linkage unit; LLVM will use this when available. (This is commonly called a “PLT stub”.) On other targets, the stub may need to be emitted explicitly.
    

This can be used wherever a `dso_local` instance of a function is needed without needing to explicitly make the original function `dso_local`. An instance where this can be used is for static offset calculations between a function and some other `dso_local` symbol. This is especially useful for the Relative VTables C++ ABI, where dynamic relocations for function pointers in VTables can be replaced with static relocations for offsets between the VTable and virtual functions which may not be `dso_local`.

This is currently only supported for ELF binary formats.

### [No CFI](https://llvm.org/docs/LangRef.html#id2044)[¶](https://llvm.org/docs/LangRef.html#no-cfi "Link to this heading")

`no_cfi @func`

With [Control-Flow Integrity (CFI)](https://clang.llvm.org/docs/ControlFlowIntegrity.html), a ‘`no_cfi`’ constant represents a function reference that does not get replaced with a reference to the CFI jump table in the `LowerTypeTests` pass. These constants may be useful in low-level programs, such as operating system kernels, which need to refer to the actual function body.

### [Pointer Authentication Constants](https://llvm.org/docs/LangRef.html#id2045)[¶](https://llvm.org/docs/LangRef.html#pointer-authentication-constants "Link to this heading")

`ptrauth (ptr CST, i32 KEY[, i64 DISC[, ptr ADDRDISC]?]?)`

A ‘`ptrauth`’ constant represents a pointer with a cryptographic authentication signature embedded into some bits, as described in the [Pointer Authentication](https://llvm.org/docs/LangRef.htmlPointerAuth.html) document.

A ‘`ptrauth`’ constant is simply a constant equivalent to the `llvm.ptrauth.sign` intrinsic, potentially fed by a discriminator `llvm.ptrauth.blend` if needed.

Its type is the same as the first argument. An integer constant discriminator and an address discriminator may be optionally specified. Otherwise, they have values `i64 0` and `ptr null`.

If the address discriminator is `null` then the expression is equivalent to

```
<span></span><span class="nv">%tmp</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">call</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="vg">@llvm.ptrauth.sign</span><span class="p">(</span><span class="kt">i64</span><span class="w"> </span><span class="k">ptrtoint</span><span class="w"> </span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="err">CST</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i64</span><span class="p">),</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="err">KEY</span><span class="p">,</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="err">DISC</span><span class="p">)</span>
<span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">inttoptr</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%tmp</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">ptr</span>
```

Otherwise, the expression is equivalent to:

```
<span></span><span class="nv">%tmp1</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">call</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="vg">@llvm.ptrauth.blend</span><span class="p">(</span><span class="kt">i64</span><span class="w"> </span><span class="k">ptrtoint</span><span class="w"> </span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="err">ADDRDISC</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i64</span><span class="p">),</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="err">DISC</span><span class="p">)</span>
<span class="nv">%tmp2</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">call</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="vg">@llvm.ptrauth.sign</span><span class="p">(</span><span class="kt">i64</span><span class="w"> </span><span class="k">ptrtoint</span><span class="w"> </span><span class="p">(</span><span class="kt">ptr</span><span class="w"> </span><span class="err">CST</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">i64</span><span class="p">),</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="err">KEY</span><span class="p">,</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%tmp1</span><span class="p">)</span>
<span class="nv">%val</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">inttoptr</span><span class="w"> </span><span class="kt">i64</span><span class="w"> </span><span class="nv">%tmp2</span><span class="w"> </span><span class="k">to</span><span class="w"> </span><span class="kt">ptr</span>
```

### [Constant Expressions](https://llvm.org/docs/LangRef.html#id2046)[¶](https://llvm.org/docs/LangRef.html#constant-expressions "Link to this heading")

Constant expressions are used to allow expressions involving other constants to be used as constants. Constant expressions may be of any [first class](https://llvm.org/docs/LangRef.html#t-firstclass) type and may involve any LLVM operation that does not have side effects (e.g. load and call are not supported). The following is the syntax for constant expressions:

`trunc (CST to TYPE)`

Perform the [trunc operation](https://llvm.org/docs/LangRef.html#i-trunc) on constants.

`ptrtoint (CST to TYPE)`

Perform the [ptrtoint operation](https://llvm.org/docs/LangRef.html#i-ptrtoint) on constants.

`ptrtoaddr (CST to TYPE)`

Perform the [ptrtoaddr operation](https://llvm.org/docs/LangRef.html#i-ptrtoaddr) on constants.

`inttoptr (CST to TYPE)`

Perform the [inttoptr operation](https://llvm.org/docs/LangRef.html#i-inttoptr) on constants. This one is _really_ dangerous!

`bitcast (CST to TYPE)`

Convert a constant, CST, to another TYPE. The constraints of the operands are the same as those for the [bitcast instruction](https://llvm.org/docs/LangRef.html#i-bitcast).

`addrspacecast (CST to TYPE)`

Convert a constant pointer or constant vector of pointer, CST, to another TYPE in a different address space. The constraints of the operands are the same as those for the [addrspacecast instruction](https://llvm.org/docs/LangRef.html#i-addrspacecast).

`getelementptr (TY, CSTPTR, IDX0, IDX1, ...)`, `getelementptr inbounds (TY, CSTPTR, IDX0, IDX1, ...)`

Perform the [getelementptr operation](https://llvm.org/docs/LangRef.html#i-getelementptr) on constants. As with the [getelementptr](https://llvm.org/docs/LangRef.html#i-getelementptr) instruction, the index list may have one or more indexes, which are required to make sense for the type of “pointer to TY”. These indexes may be implicitly sign-extended or truncated to match the index size of CSTPTR’s address space.

`extractelement (VAL, IDX)`

Perform the [extractelement operation](https://llvm.org/docs/LangRef.html#i-extractelement) on constants.

`insertelement (VAL, ELT, IDX)`

Perform the [insertelement operation](https://llvm.org/docs/LangRef.html#i-insertelement) on constants.

`shufflevector (VEC1, VEC2, IDXMASK)`

Perform the [shufflevector operation](https://llvm.org/docs/LangRef.html#i-shufflevector) on constants.

`add (LHS, RHS)`

Perform an addition on constants.

`sub (LHS, RHS)`

Perform a subtraction on constants.

`xor (LHS, RHS)`

Perform a bitwise xor on constants.

## [Other Values](https://llvm.org/docs/LangRef.html#id2047)[¶](https://llvm.org/docs/LangRef.html#other-values "Link to this heading")

### [Inline Assembler Expressions](https://llvm.org/docs/LangRef.html#id2048)[¶](https://llvm.org/docs/LangRef.html#inline-assembler-expressions "Link to this heading")

LLVM supports inline assembler expressions (as opposed to [Module-Level Inline Assembly](https://llvm.org/docs/LangRef.html#moduleasm)) through the use of a special value. This value represents the inline assembler as a template string (containing the instructions to emit), a list of operand constraints (stored as a string), a flag that indicates whether or not the inline asm expression has side effects, and a flag indicating whether the function containing the asm needs to align its stack conservatively.

The template string supports argument substitution of the operands using “`$`” followed by a number, to indicate substitution of the given register/memory location, as specified by the constraint string. “`${NUM:MODIFIER}`” may also be used, where `MODIFIER` is a target-specific annotation for how to print the operand (See [Asm template argument modifiers](https://llvm.org/docs/LangRef.html#inline-asm-modifiers)).

A literal “`$`” may be included by using “`$$`” in the template. To include other special characters into the output, the usual “`\XX`” escapes may be used, just as in other strings. Note that after template substitution, the resulting assembly string is parsed by LLVM’s integrated assembler unless it is disabled – even when emitting a `.s` file – and thus must contain assembly syntax known to LLVM.

LLVM also supports a few more substitutions useful for writing inline assembly:

-   `${:uid}`: Expands to a decimal integer unique to this inline assembly blob. This substitution is useful when declaring a local label. Many standard compiler optimizations, such as inlining, may duplicate an inline asm blob. Adding a blob-unique identifier ensures that the two labels will not conflict during assembly. This is used to implement [GCC’s %= special format string](https://gcc.gnu.org/onlinedocs/gcc/Extended-Asm.html).
    
-   `${:comment}`: Expands to the comment character of the current target’s assembly dialect. This is usually `#`, but many targets use other strings, such as `;`, `//`, or `!`.
    
-   `${:private}`: Expands to the assembler private label prefix. Labels with this prefix will not appear in the symbol table of the assembled object. Typically the prefix is `L`, but targets may use other strings. `.L` is relatively popular.
    

LLVM’s support for inline asm is modeled closely on the requirements of Clang’s GCC-compatible inline-asm support. Thus, the feature-set and the constraint and modifier codes listed here are similar or identical to those in GCC’s inline asm support. However, to be clear, the syntax of the template and constraint strings described here is _not_ the same as the syntax accepted by GCC and Clang, and, while most constraint letters are passed through as-is by Clang, some get translated to other codes when converting from the C source to the LLVM assembly.

An example inline assembler expression is:

```
<span></span><span class="kt">i32</span><span class="w"> </span><span class="p">(</span><span class="kt">i32</span><span class="p">)</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="s">"bswap $0"</span><span class="p">,</span><span class="w"> </span><span class="s">"=r,r"</span>
```

Inline assembler expressions may **only** be used as the callee operand of a [call](https://llvm.org/docs/LangRef.html#i-call) or an [invoke](https://llvm.org/docs/LangRef.html#i-invoke) instruction. Thus, typically we have:

```
<span></span><span class="nv">%X</span><span class="w"> </span><span class="p">=</span><span class="w"> </span><span class="k">call</span><span class="w"> </span><span class="kt">i32</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="s">"bswap $0"</span><span class="p">,</span><span class="w"> </span><span class="s">"=r,r"</span><span class="p">(</span><span class="kt">i32</span><span class="w"> </span><span class="nv">%Y</span><span class="p">)</span>
```

Inline asms with side effects not visible in the constraint list must be marked as having side effects. This is done through the use of the ‘`sideeffect`’ keyword, like so:

```
<span></span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="k">sideeffect</span><span class="w"> </span><span class="s">"eieio"</span><span class="p">,</span><span class="w"> </span><span class="s">""</span><span class="p">()</span>
```

In some cases inline asms will contain code that will not work unless the stack is aligned in some way, such as calls or SSE instructions on x86, yet will not contain code that does that alignment within the asm. The compiler should make conservative assumptions about what the asm might contain and should generate its usual stack alignment code in the prologue if the ‘`alignstack`’ keyword is present:

```
<span></span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="k">alignstack</span><span class="w"> </span><span class="s">"eieio"</span><span class="p">,</span><span class="w"> </span><span class="s">""</span><span class="p">()</span>
```

Inline asms also support using non-standard assembly dialects. The assumed dialect is ATT. When the ‘`inteldialect`’ keyword is present, the inline asm is using the Intel dialect. Currently, ATT and Intel are the only supported dialects. An example is:

```
<span></span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="k">inteldialect</span><span class="w"> </span><span class="s">"eieio"</span><span class="p">,</span><span class="w"> </span><span class="s">""</span><span class="p">()</span>
```

In the case that the inline asm might unwind the stack, the ‘`unwind`’ keyword must be used, so that the compiler emits unwinding information:

```
<span></span><span class="k">call</span><span class="w"> </span><span class="k">void</span><span class="w"> </span><span class="k">asm</span><span class="w"> </span><span class="k">unwind</span><span class="w"> </span><span class="s">"call func"</span><span class="p">,</span><span class="w"> </span><span class="s">""</span><span class="p">()</span>
```

If the inline asm unwinds the stack and isn’t marked with the ‘`unwind`’ keyword, the behavior is undefined.

If multiple keywords appear, the ‘`sideeffect`’ keyword must come first, the ‘`alignstack`’ keyword second, the ‘`inteldialect`’ keyword third, and the ‘`unwind`’ keyword last.

#### [Inline Asm Constraint String](https://llvm.org/docs/LangRef.html#id2049)[¶](https://llvm.org/docs/LangRef.html#inline-asm-constraint-string "Link to this heading")

The constraint list is a comma-separated string, each element containing one or more constraint codes.

For each element in the constraint list an appropriate register or memory operand will be chosen, and it will be made available to assembly template string expansion as `$0` for the first constraint in the list, `$1` for the second, etc.

There are three different types of constraints, which are distinguished by a prefix symbol in front of the constraint code: Output, Input, and Clobber. The constraints must always be given in that order: outputs first, then inputs, then clobbers. They cannot be intermingled.

There are also three different categories of constraint codes:

-   Register constraint. This is either a register class, or a fixed physical register. This kind of constraint will allocate a register, and if necessary, bitcast the argument or result to the appropriate type.
    
-   Memory constraint. This kind of constraint is for use with an instruction taking a memory operand. Different constraints allow for different addressing modes used by the target.
    
-   Immediate value constraint. This kind of constraint is for an integer or other immediate value which can be rendered directly into an instruction. The various target-specific constraints allow the selection of a value in the proper range for the instruction you wish to use it with.
    

##### Output constraints[¶](https://llvm.org/docs/LangRef.html#output-constraints "Link to this heading")

Output constraints are specified by an “`=`” prefix (e.g. “`=r`”). This indicates that the assembly will write to this operand, and the operand will then be made available as a return value of the `asm` expression. Output constraints do not consume an argument from the call instruction. (Except, see below about indirect outputs).

Normally, it is expected that no output locations are written to by the assembly expression until _all_ of the inputs have been read. As such, LLVM may assign the same register to an output and an input. If this is not safe (e.g. if the assembly contains two instructions, where the first writes to one output, and the second reads an input and writes to a second output), then the “`&`” modifier must be used (e.g. “`=&r`”) to specify that the output is an “early-clobber” output. Marking an output as “early-clobber” ensures that LLVM will not use the same register for any inputs (other than an input tied to this output).

##### Input constraints[¶](https://llvm.org/docs/LangRef.html#input-constraints "Link to this heading")

Input constraints do not have a prefix – just the constraint codes. Each input constraint will consume one argument from the call instruction. It is not permitted for the asm to write to any input register or memory location (unless that input is tied to an output). Note also that multiple inputs may all be assigned to the same register, if LLVM can determine that they necessarily all contain the same value.

Instead of providing a Constraint Code, input constraints may also “tie” themselves to an output constraint, by providing an integer as the constraint string. Tied inputs still consume an argument from the call instruction, and take up a position in the asm template numbering as is usual – they will simply be constrained to always use the same register as the output they’ve been tied to. For example, a constraint string of “`=r,0`” says to assign a register for output, and use that register as an input as well (it being the 0’th constraint).

It is permitted to tie an input to an “early-clobber” output. In that case, no _other_ input may share the same register as the input tied to the early-clobber (even when the other input has the same value).

You may only tie an input to an output which has a register constraint, not a memory constraint. Only a single input may be tied to an output.

There is also an “interesting” feature which deserves a bit of explanation: if a register class constraint allocates a register which is too small for the value type operand provided as input, the input value will be split into multiple registers, and all of them passed to the inline asm.

However, this feature is often not as useful as you might think.

Firstly, the registers are _not_ guaranteed to be consecutive. So, on those architectures that have instructions which operate on multiple consecutive instructions, this is not an appropriate way to support them. (e.g. the 32-bit SparcV8 has a 64-bit load, which instruction takes a single 32-bit register. The hardware then loads into both the named register, and the next register. This feature of inline asm would not be useful to support that.)

A few of the targets provide a template string modifier allowing explicit access to the second register of a two-register operand (e.g. MIPS `L`, `M`, and `D`). On such an architecture, you can actually access the second allocated register (yet, still, not any subsequent ones). But, in that case, you’re still probably better off simply splitting the value into two separate operands, for clarity. (e.g. see the description of the `A` constraint on X86, which, despite existing only for use with this feature, is not really a good idea to use)

##### Indirect inputs and outputs[¶](https://llvm.org/docs/LangRef.html#indirect-inputs-and-outputs "Link to this heading")

Indirect output or input constraints can be specified by the “`*`” modifier (which goes after the “`=`” in case of an output). This indicates that the asm will write to or read from the contents of an _address_ provided as an input argument. (Note that in this way, indirect outputs act more like an _input_ than an output: just like an input, they consume an argument of the call expression, rather than producing a return value. An indirect output constraint is an “output” only in that the asm is expected to write to the contents of the input memory location, instead of just read from it).

This is most typically used for memory constraint, e.g. “`=*m`”, to pass the address of a variable as a value.

It is also possible to use an indirect _register_ constraint, but only on output (e.g. “`=*r`”). This will cause LLVM to allocate a register for an output value normally, and then, separately emit a store to the address provided as input, after the provided inline asm. (It’s not clear what value this functionality provides, compared to writing the store explicitly after the asm statement, and it can only produce worse code, since it bypasses many optimization passes. I would recommend not using it.)

Call arguments for indirect constraints must have pointer type and must specify the [elementtype](https://llvm.org/docs/LangRef.html#attr-elementtype) attribute to indicate the pointer element type.

##### Clobber constraints[¶](https://llvm.org/docs/LangRef.html#clobber-constraints "Link to this heading")

A clobber constraint is indicated by a “`~`” prefix. A clobber does not consume an input operand, nor generate an output. Clobbers cannot use any of the general constraint code letters – they may use only explicit register constraints, e.g. “`~{eax}`”. The one exception is that a clobber string of “`~{memory}`” indicates that the assembly writes to arbitrary undeclared memory locations – not only the memory pointed to by a declared indirect output.

Note that clobbering named registers that are also present in output constraints is not legal.

##### Label constraints[¶](https://llvm.org/docs/LangRef.html#label-constraints "Link to this heading")

A label constraint is indicated by a “`!`” prefix and typically used in the form `"!i"`. Instead of consuming call arguments, label constraints consume indirect destination labels of `callbr` instructions.

Label constraints can only be used in conjunction with `callbr` and the number of label constraints must match the number of indirect destination labels in the `callbr` instruction.

##### Constraint Codes[¶](https://llvm.org/docs/LangRef.html#constraint-codes "Link to this heading")

After a potential prefix comes constraint code, or codes.

A Constraint Code is either a single letter (e.g. “`r`”), a “`^`” character followed by two letters (e.g. “`^wc`”), or “`{`” register-name “`}`” (e.g. “`{eax}`”).

The one and two letter constraint codes are typically chosen to be the same as GCC’s constraint codes.

A single constraint may include one or more constraint codes in it, leaving it up to LLVM to choose which one to use. This is included mainly for compatibility with the translation of GCC inline asm coming from clang.

There are two ways to specify alternatives, and either or both may be used in an inline asm constraint list:

1.  Append the codes to each other, making a constraint code set. E.g. “`im`” or “`{eax}m`”. This means “choose any of the options in the set”. The choice of constraint is made independently for each constraint in the constraint list.
    
2.  Use “`|`” between constraint code sets, creating alternatives. Every constraint in the constraint list must have the same number of alternative sets. With this syntax, the same alternative in _all_ of the items in the constraint list will be chosen together.
    

Putting those together, you might have a two operand constraint string like `"rm|r,ri|rm"`. This indicates that if operand 0 is `r` or `m`, then operand 1 may be one of `r` or `i`. If operand 0 is `r`, then operand 1 may be one of `r` or `m`. But, operand 0 and 1 cannot both be of type m.

However, the use of either of the alternatives features is _NOT_ recommended, as LLVM is not able to make an intelligent choice about which one to use. (At the point it currently needs to choose, not enough information is available to do so in a smart way.) Thus, it simply tries to make a choice that’s most likely to compile, not one that will be optimal performance. (e.g., given “`rm`”, it’ll always choose to use memory, not registers). And, if given multiple registers, or multiple register classes, it will simply choose the first one. (In fact, it doesn’t currently even ensure explicitly specified physical registers are unique, so specifying multiple physical registers as alternatives, like `{r11}{r12},{r11}{r12}`, will assign r11 to both operands, not at all what was intended.)
