> __GBC__ - Go B Compiler - https://github.com/xplshn/gbc
>
> A much more capable B compiler, written using modernc.org/libqbe
---

# (gbc) | The Go B Compiler

This compiler is a project aiming to make a valid B compiler, with _optional_ extensions for C interoperability, and a modules system like Go's

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
    -h, --help             Display this information.
    -std=<std>             Specify language standard (B, Bx). Default: Bx
    -pedantic              Issue all warnings demanded by the current B std.

  Warning Flags
    -Wall                  Enable most warnings.
    -Wno-all               Disable all warnings.
    -W<warning>            Enable a specific warning.
    -Wno-<warning>         Disable a specific warning.
    Available warnings:
      c-escapes            Using C-style '\' escapes instead of B's '*'                                [x]
      b-escapes            Using historical B-style '*' escapes instead of C's '\'                     [x]
      b-ops                Using historical B assignment operators like '=+'                           [x]
      c-ops                Using C-style assignment operators like '+=' in -std=B mode                 [x]
      unrecognized-escape  Using unrecognized escape sequences                                         [x]
      truncated-char       Character escape value is too large for a byte and has been truncated       [x]
      long-char-const      Multi-character constant is too long for a word                             [x]
      c-comments           Using non-standard C-style '//' comments                                    [-]
      overflow             Integer constant is out of range for its type                               [x]
      pedantic             Issues that violate the current strict -std=                                [-]
      unreachable-code     Unreachable code                                                            [x]
      extra                Extra warnings (e.g., poor choices, unrecognized flags)                     [x]

  Feature Flags
    -F<feature>            Enable a specific feature.
    -Fno-<feature>         Disable a specific feature.
    Available features:
      extrn                Allow the 'extrn' keyword                                                   [x]
      asm                  Allow the '__asm__' keyword and blocks                                      [x]
      b-escapes            Recognize B-style '*' character escapes                                     [x]
      c-escapes            Recognize C-style '\' character escapes                                     [x]
      b-ops                Recognize B-style assignment operators like '=+'                            [x]
      c-ops                Recognize C-style assignment operators like '+='                            [x]
      c-comments           Recognize C-style '//' comments                                             [x]

]~/Documents/TrulyMine/gbc@ 
```

#### Progress Report:
- Capable of compiling donut.b
- Capable of compiling snake.b, but collisions for some reason do not work against the apple
- Capable of passing all tests
- Most examples work, but some fail to work as expected, with slight differences in behavior - I'm working on fixing this issue

The project is currently in its infancy, and the long-term goals are very ambitious. This is the current roadmap:

> ##### ROADMAP
>
> ###### (i) Tests
> * ~~Make a script that takes the tests from [tsoding/b](https://github.com/tsoding/b), and filters the tests.json to only include the IR tests~~
> * Make a Go program that runs each test, displays the passing/failing ones
>
> ###### (ii) Compatibility with [tsoding/b](https://github.com/tsoding/b)
> 1. ~~Support the "extrn" keyword, as well as inline assembly~~
> 2. ~~Use the same warning & error messages [tsoding/b](https://github.com/tsoding/b)~~ / our warnings n errors are much better
> 3. ~~Be able to pass the IR tests of [tsoding/b](https://github.com/tsoding/b)~~
>
> ###### (iii) Packages / Modules inspired by Go
> * ¿.. Namespaces based on .mod file ..?
> * Implement a way to import/export symbols from different .B files, in different namespaces
>

### Contributions are hyper-mega welcome

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
- [cproc](https://github.com/michaelforney/cproc): QBE-backed compiler written in C99, with support for some C23 & GNU C extensions

###### Cool stuff used by this project:
- [QBE](https://c9x.me/compile/): The QBE compiler backend

---

###### TODO: ...I should write a Limbo compiler when I finish this project...
