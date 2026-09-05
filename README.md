# hyper_copy

`hyper_copy` is a command-line tool that performs text and filename replacement while copying files.
The "Preserve Case" feature is enabled by default, which automatically adjusts the case of the replacement string to match the case of the original matched string (e.g., camelCase, PascalCase, UPPERCASE, lowercase).

*Note: For the Japanese documentation, please refer to [README.ja.md](README.ja.md).*

## Features

- String replacement while copying file contents.
- Support for specifying multiple replacement rules simultaneously (`--from`, `--to`, `--from2`, `--to2`, etc.).
- Automatically detects and preserves the casing of the target string (UPPERCASE, lowercase, PascalCase, camelCase).
- Batch copy to a directory (filenames are also automatically replaced).
- Safe overwrite protection to prevent accidental data loss (aborts safely via pre-checks).

## Prerequisites

- Windows environment with Ruby installed.

## Installation

Place the following files from this directory (`hyper_copy`) into a directory that is included in your system's `PATH` environment variable. This will allow you to run the `hyper_copy` command from anywhere.

- `hyper_copy.rb` (Core script)
- `hyper_copy.cmd` (Command execution wrapper for Windows)

## Usage

### Basic Usage

```sh
hyper_copy --from FooBar --to AaaBbb Foo.cs Aaa.cs
```
This replaces strings related to `FooBar` with `AaaBbb` in the contents of `Foo.cs` and saves the result as `Aaa.cs`.

### Specifying Multiple Replacement Rules

You can specify `--from` and `--to` multiple times. For clarity, you can also append numbers such as `--from2` and `--to2`.

```sh
hyper_copy --from FooBar --to AaaBbb --from2 フー --to2 バー Foo.cs Aaa.cs
```

### Batch Copying to a Directory

If you specify an existing directory as the last argument, you can copy multiple files at once.
In this case, **the replacement rules are also applied to the filenames**.

```sh
hyper_copy --from Foo --to Aaa Foo.cs Bar.cs ../tmp
```
In this scenario, the files are copied as follows:
- `Foo.cs` -> `../tmp/Aaa.cs` (Both filename and contents are replaced)
- `Bar.cs` -> `../tmp/Bar.cs` (Only contents are replaced)

### Overwrite Protection and Forcing Overwrite (-f)

By default, if even one file with the same name already exists in the destination, **the entire process will fail without modifying any files**.

```sh
$ hyper_copy --from Foo --to Aaa Foo.cs Bar.cs .
Cannot overwrite existing file(s): Aaa.cs
Use -f or --force to overwrite.
```

If you want to overwrite existing files, add the `-f` or `--force` option.

```sh
$ hyper_copy -f --from Foo --to Aaa Foo.cs Bar.cs .
Foo.cs -> Aaa.cs
Bar.cs -> Bar.cs (overwrite)
```

## About Preserve Case

When the replacement target (the string specified with `--to`) is `AaaBbb`, the matched string will be smartly replaced based on its original casing as follows:

| String in Original File | Replaced String | Notes |
| --- | --- | --- |
| `FooBar` | `AaaBbb` | PascalCase (Matches capitalized first letter) |
| `fooBar` | `aaaBbb` | camelCase (Matches lowercase first letter) |
| `FOOBAR` | `AAABBB` | ALL CAPS |
| `foobar` | `aaabbb` | all lowercase |

*Note: For characters that do not have upper/lower cases (such as Japanese characters, e.g., `フー` -> `バー`), they are replaced exactly as specified.*

For more detailed conversion rules and compatibility with Visual Studio Code's behavior, please refer to [doc/preserve_case.md](doc/preserve_case.md) (Japanese only).

## Benchmark

Performance comparison of different language implementations (copying and replacing text in 10 large files). You can run `ruby test/benchmark.rb` to reproduce this test.

| Language | Execution Time (s) | Binary Size |
| --- | --- | --- |
| Rust | 1.669 s | 1.80 MB |
| Go | 2.321 s | 3.00 MB |
| C# (Standard) | 2.590 s | 144.00 KB |
| Ruby | 3.353 s | N/A (Script) |
| C# (Self-Contained) | 4.679 s | 67.65 MB |
