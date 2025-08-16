> __CBC__ - C B Compiler - https://github.com/xplshn/cbc
>
> The predecesor of this B compiler, written in C11 and using QBE as its backend
---

# (gbc) | The Go B Compiler

This compiler is a project aiming to make a valid B compiler, with _optional_ syntax extensions, and a modules system like Go's

```
]~/Documents/TrulyMine/gbc@ ./gbc --help

Copyright (c) 2025: xplshn and contributors
For more details refer to <https://github.com/xplshn/gbc>

  Synopsis
    gbc [options] <input.b> ...

  Description
    A compiler for the B programming language and its extensions, written in Go.

  Options
    -o <file>              Place the output into <file>.
    -t, --target <target>  Set the QBE target ABI.
    -I <path>              Add a directory to the include path.
    -L <arg>               Pass an argument to the linker.
    -C <arg>               Pass a compiler-specific argument (e.g., -C linker_args='-s').
    -l<lib>                Link with a library (e.g., -lb for 'b').
    -h, --help             Display this information.
    -std=<std>             Specify language standard (B, Bx). Default: Bx
    -pedantic              Issue all warnings demanded by the current B std.

  Warning Flags
    -Wall                  Enable most warnings.
    -Wno-all               Disable all warnings.
    -W<warnings>           Enable a specific warnings.
    -Wno-<warnings>        Disable a specific warnings.
    Available warnings:
  c-esc                Warn on usage of C-style '\' escapes.                                       [x]
  b-esc                Warn on usage of B-style '*' escapes.                                       [x]
  b-ops                Warn on usage of B-style assignment operators like '=+'.                    [x]
  c-ops                Warn on usage of C-style assignment operators like '+='.                    [x]
  u-esc                Warn on unrecognized character escape sequences.                            [x]
  truncated-char       Warn when a character escape value is truncated.                            [x]
  long-char-const      Warn when a multi-character constant is too long for a word.                [x]
  c-comments           Warn on usage of non-standard C-style '//' comments.                        [-]
  overflow             Warn when an integer constant is out of range for its type.                 [x]
  pedantic             Issue all warnings demanded by the strict standard.                         [-]
  unreachable-code     Warn about code that will never be executed.                                [x]
  implicit-decl        Warn about implicit function or variable declarations.                      [x]
  type                 Warn about type mismatches in expressions and assignments.                  [x]
  extra                Enable extra miscellaneous warnings.                                        [x]

  Feature Flags
    -F<features>           Enable a specific features.
    -Fno-<features>        Disable a specific features.
    Available features:
  extrn                Allow the 'extrn' keyword.                                                  [x]
  asm                  Allow `__asm__` blocks for inline assembly.                                 [x]
  b-esc                Recognize B-style '*' character escapes.                                    [-]
  c-esc                Recognize C-style '\' character escapes.                                    [x]
  b-ops                Recognize B-style assignment operators like '=+'.                           [-]
  c-ops                Recognize C-style assignment operators like '+='.                           [x]
  c-comments           Recognize C-style '//' line comments.                                       [x]
  typed                Enable the Bx opt-in & backwards-compatible type system.                    [x]
  short-decl           Enable Bx-style short declaration `:=`.                                     [x]
  bx-decl              Enable Bx-style `auto name = val` declarations.                             [x]
  allow-uninitialized  Allow declarations without an initializer (`var;` or `auto var;`).          [x]
  strict-decl          Require all declarations to be initialized.                                 [-]
  no-directives        Disable `// [b]:` directives.                                               [-]
  continue             Allow the Bx keyword `continue` to be used.                                 [x]

]~/Documents/TrulyMine/gbc@ 
```

### Progress Report:
- Capable of passing all tests
- Capable of compiling donut.b
- Capable of compiling snake.b
- Capable of compiling raylib.b (just remember to link with Raylib via `-L -lraylib`)
- Capable of compiling langtons_ants.b
- Capable of compiling brainfck.b
- Etc, these are just the most impressive examples
- I added a completely opt-in type system. It uses type first declarations like C, and uses the Go type names. (can also be used with strict B via `-std=B -Ftyped`, the syntax is backwards compatible. Its so reliable it comes enabled by default.)
- `gbc` will warn about poorly written code. TODO: Convert these warnings into annotations that offer suggestions.
- I'm working on adding support for alternative backends. QBE will remain as the default, but there should be other options as well, including a C one. (TODO: Expose GOABI0 target that libQBE provides)

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
> 4. A gameboy color target once _all_ examples can be compiled and work as expected
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
