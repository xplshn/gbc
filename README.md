> __CBC__ - C B Compiler - https://github.com/xplshn/cbc
>
> The predecesor of this B compiler, written in C11 and using QBE as its backend
---

# (gbc) | The Go B Compiler

This compiler is a project aiming to make a valid B compiler, with _optional_ syntax extensions, and a modules system like Go's

```sh
]~/Documents/TrulyMine/gbc@ ./gbc --help

    Copyright (c) 2025: xplshn and contributors
    For more details refer to <https://github.com/xplshn/gbc>

    Synopsis
        gbc <options> <input.b> ...

    Description
        A compiler for the B programming language with modern extensions. Like stepping into a time machine, but with better error messages.

    Options
        -C <arg>, --compiler-arg <arg>                 Pass a compiler-specific argument (e.g., -C linker_args='-s').
        -d, --dump-ir                                  Dump the intermediate representation and exit.
        -h, --help                                     Display this information
        -I <path>, --include <path>                    Add a directory to the include path.
        -L <arg>, --linker-arg <arg>                   Pass an argument to the linker.
        -o <file>, --output <file>                     Place the output into <file>.                                          |a.out|
        --pedantic                                     Issue all warnings demanded by the current B std.
        --std=std                                      Specify language standard (B, Bx)                                      |Bx|
        -t <backend/target>, --target <backend/target> Set the backend and target ABI.                                        |qbe|

    Feature Flags
        -F<feature flag>                               Enable a specific feature flag
        -Fno-<feature flag>                            Disable a specific feature flag
    Available feature flags:
        allow-uninitialized                            Allow declarations without an initializer (`var;` or `auto var;`)      |x|
        asm                                            Allow `__asm__` blocks for inline assembly                             |x|
        b-esc                                          Recognize B-style '*' character escapes                                |-|
        b-ops                                          Recognize B-style assignment operators like '=+'                       |-|
        bx-decl                                        Enable Bx-style `auto name = val` declarations                         |x|
        c-comments                                     Recognize C-style '//' line comments                                   |x|
        c-esc                                          Recognize C-style '\' character escapes                                |x|
        c-ops                                          Recognize C-style assignment operators like '+='                       |x|
        continue                                       Allow the Bx keyword `continue` to be used                             |x|
        extrn                                          Allow the 'extrn' keyword                                              |x|
        float                                          Enable support for floating-point numbers                              |x|
        no-directives                                  Disable `// [b]:` directives                                           |-|
        prom-types                                     Enable type promotions - promote untyped literals to compatible types  |-|
        short-decl                                     Enable Bx-style short declaration `:=`                                 |x|
        strict-decl                                    Require all declarations to be initialized                             |-|
        strict-types                                   Disallow all incompatible type operations                              |-|
        typed                                          Enable the Bx opt-in & backwards-compatible type system                |x|

    Warning Flags
        -W<warning flag>                               Enable a specific warning flag
        -Wno-<warning flag>                            Disable a specific warning flag
    Available Warning Flags:
        b-esc                                          Warn on usage of B-style '*' escapes                                   |x|
        b-ops                                          Warn on usage of B-style assignment operators like '=+'                |x|
        c-comments                                     Warn on usage of non-standard C-style '//' comments                    |-|
        c-esc                                          Warn on usage of C-style '\' escapes                                   |-|
        c-ops                                          Warn on usage of C-style assignment operators like '+='                |-|
        debug-comp                                     Debug warning for type promotions and conversions                      |-|
        extra                                          Enable extra miscellaneous warnings                                    |x|
        float                                          Warn when floating-point numbers are used                              |-|
        implicit-decl                                  Warn about implicit function or variable declarations                  |x|
        local-address                                  Warn when the address of a local variable is returned                  |x|
        long-char-const                                Warn when a multi-character constant is too long for a word            |x|
        overflow                                       Warn when an integer constant is out of range for its type             |x|
        pedantic                                       Issue all warnings demanded by the strict standard                     |-|
        prom-types                                     Warn when type promotions occur                                        |x|
        truncated-char                                 Warn when a character escape value is truncated                        |x|
        u-esc                                          Warn on unrecognized character escape sequences                        |x|
        unreachable-code                               Warn about code that will never be executed                            |x|
]~/Documents/TrulyMine/gbc@
```

### Progress Report:
- Capable of passing all tests
- Capable of compiling all examples. Producing the same output as the reference B compiler, against the same STDIN and argument inputs.
- Etc, these are just the most impressive examples
- I added a completely opt-in type system. It uses type first declarations like C, and uses the Go type names. (can also be used with strict B via `-std=B -Ftyped`, the syntax is backwards compatible. Its so reliable it comes enabled by default.)
- `gbc`'s warnings warn against common errors, poor decisions, etc
- Directives are supported
- Meta-programming: W.I.P
- Borrow-checking: Working on that!!! Will probably be the last feature of GBC once the most essential stuff is addressed
- Portable and with multiple backends:
  - QBE (default, via modernc.org/libQBE, a pure Go version of QBE)
  - LLVM (via `llc`)
  - TODO: GameBoy Color Target... Coming soon!!!

## Demo
<img width="1920" height="1080" alt="RayLib B demo" src="https://github.com/user-attachments/assets/ed941fc1-0754-4978-98fb-13ff2774b880" />
<img width="1920" height="1080" alt="Snake.b" src="https://github.com/user-attachments/assets/d235166b-06c3-4849-859f-1e97f44e15af" />
<img width="1920" height="1080" alt="image" src="https://github.com/user-attachments/assets/07b79046-75cd-4014-a870-54a10f6853bd" />

###### TODO: GIFs showcasing things properly

The project is currently in its infancy, and the long-term goals are very ambitious. This is the current roadmap:

> ##### ROADMAP
>
> ###### (i) Tests
> * ~~Make a script that takes the tests from [tsoding/b](https://github.com/tsoding/b), and filters the tests.json to only include the IR tests~~
> * ~~Make a Go program that runs each test, displays the passing/failing ones~~
>
> ###### (ii) Compatibility with [tsoding/b](https://github.com/tsoding/b)
> 1. ~~Support the "extrn" keyword, as well as inline assembly~~
> 2. ~~Use the same warning & error messages [tsoding/b](https://github.com/tsoding/b)~~ / our warnings n errors are much better
> 3. ~~Be able to pass the IR tests of [tsoding/b](https://github.com/tsoding/b)~~
> 4. A gameboy color target once _all_ examples can be compiled and work as expected (WIP)
>
> ###### (iii) Packages / Modules inspired by Go
> * ¿.. Namespaces based on .mod file ..?
> * Implement a way to import/export symbols from different .B files, in different namespaces
>

### Contributions are hyper-mega welcome

---

#### Acknowledgments

##### References
- https://research.swtch.com/b-lang
- https://www.nokia.com/bell-labs/about/dennis-m-ritchie/kbman.html
- https://www.nokia.com/bell-labs/about/dennis-m-ritchie/bref.html
- https://www.nokia.com/bell-labs/about/dennis-m-ritchie/btut.html
- https://github.com/Spydr06/BCause
- https://github.com/kparc/bcc
###### Not B-related, but I did find these helpful for learning how to write the compiler:
- [comp-bib](https://c9x.me/compile/bib/): "Resources for Amateur Compiler Writers"
- [qcc](https://c9x.me/qcc): dead-simple C compiler
- [scc](https://www.simple-cc.org): The best C99 compiler out there. QBE-backed.
- [cproc](https://github.com/michaelforney/cproc): QBE-backed compiler written in C99, with support for some C23 & GNU C extensions

###### Cool stuff used by this project:
- [QBE](https://c9x.me/compile/): The QBE compiler backend
- [modernc.org/libqbe](modernc.org/libqbe): A pure-go convertion of QBE, this allows `gbc` to be self-contained

---

###### TODO: ...I should write a Limbo compiler when I finish this project...
