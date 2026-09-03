use regex::{Regex, RegexBuilder};
use std::env;
use std::fs;
use std::path::Path;
use std::process;

fn ruby_capitalize(s: &str) -> String {
    if s.is_empty() {
        return String::new();
    }
    let mut chars = s.chars();
    let first = chars.next().unwrap().to_uppercase().to_string();
    let rest: String = chars.map(|c| c.to_lowercase().to_string()).collect();
    format!("{}{}", first, rest)
}

fn preserve_case(match_str: &str, to_str: &str) -> String {
    if to_str.is_empty() {
        return String::new();
    }

    let match_upper = match_str.to_uppercase();
    let match_lower = match_str.to_lowercase();
    let match_cap = ruby_capitalize(match_str);

    if match_str == match_upper {
        return to_str.to_uppercase();
    } else if match_str == match_lower {
        return to_str.to_lowercase();
    } else if match_str == match_cap {
        return ruby_capitalize(to_str);
    } else {
        let mut match_chars = match_str.chars();
        let mut to_chars = to_str.chars();
        if let (Some(first_match), Some(first_to)) = (match_chars.next(), to_chars.next()) {
            let rest_to: String = to_chars.collect();
            if first_match.is_lowercase() {
                return format!("{}{}", first_to.to_lowercase(), rest_to);
            } else {
                return format!("{}{}", first_to.to_uppercase(), rest_to);
            }
        }
    }
    to_str.to_string()
}

fn apply_replacements(text: &str, replacements: &[(String, String)]) -> String {
    if replacements.is_empty() {
        return text.to_string();
    }

    let mut sorted_pairs = replacements.to_vec();
    sorted_pairs.sort_by(|a, b| b.0.len().cmp(&a.0.len()));

    let pattern_parts: Vec<String> = sorted_pairs
        .iter()
        .map(|p| regex::escape(&p.0))
        .collect();
    let pattern = format!("(?i)({})", pattern_parts.join("|"));
    let re = RegexBuilder::new(&pattern)
        .build()
        .unwrap();

    re.replace_all(text, |caps: &regex::Captures| {
        let match_str = caps.get(0).unwrap().as_str();
        let mut to_str = "";
        for p in &sorted_pairs {
            if p.0.eq_ignore_ascii_case(match_str) {
                to_str = &p.1;
                break;
            }
        }
        preserve_case(match_str, to_str)
    })
    .to_string()
}

struct ArgSuffixPair {
    suffix: String,
    val: String,
}

fn main() {
    let mut force = false;
    let mut args_list = Vec::new();
    let mut from_list: Vec<ArgSuffixPair> = Vec::new();
    let mut to_list: Vec<ArgSuffixPair> = Vec::new();

    let from_re = Regex::new(r"^--from(\d*)$").unwrap();
    let to_re = Regex::new(r"^--to(\d*)$").unwrap();

    let args: Vec<String> = env::args().skip(1).collect();
    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];
        if arg == "-f" || arg == "--force" {
            force = true;
            i += 1;
        } else if let Some(caps) = from_re.captures(arg) {
            let suffix = caps.get(1).unwrap().as_str().to_string();
            if i + 1 < args.len() {
                from_list.push(ArgSuffixPair {
                    suffix,
                    val: args[i + 1].clone(),
                });
                i += 2;
            } else {
                eprintln!("Error: Missing value for {}", arg);
                process::exit(1);
            }
        } else if let Some(caps) = to_re.captures(arg) {
            let suffix = caps.get(1).unwrap().as_str().to_string();
            if i + 1 < args.len() {
                to_list.push(ArgSuffixPair {
                    suffix,
                    val: args[i + 1].clone(),
                });
                i += 2;
            } else {
                eprintln!("Error: Missing value for {}", arg);
                process::exit(1);
            }
        } else {
            args_list.push(arg.clone());
            i += 1;
        }
    }

    if args_list.len() < 2 {
        eprintln!("Usage: hyper_copy [options] <source...> <dest>");
        process::exit(1);
    }

    let mut replacements = Vec::new();
    for f in &from_list {
        if let Some(t_idx) = to_list.iter().position(|t| t.suffix == f.suffix) {
            replacements.push((f.val.clone(), to_list[t_idx].val.clone()));
            to_list.remove(t_idx);
        } else {
            eprintln!(
                "Error: Missing --to{} for --from{} {}",
                f.suffix, f.suffix, f.val
            );
            process::exit(1);
        }
    }

    if !to_list.is_empty() {
        eprintln!(
            "Error: Missing --from{} for --to{} {}",
            to_list[0].suffix, to_list[0].suffix, to_list[0].val
        );
        process::exit(1);
    }

    let dest = args_list.pop().unwrap();
    let sources = args_list;

    let mut tasks = Vec::new();

    let dest_path = Path::new(&dest);
    if dest_path.is_dir() {
        for src in sources {
            let src_path = Path::new(&src);
            if let Some(file_name) = src_path.file_name() {
                let file_name_str = file_name.to_string_lossy().to_string();
                let new_basename = apply_replacements(&file_name_str, &replacements);
                let target_path = dest_path.join(new_basename);
                tasks.push((src, target_path.to_string_lossy().to_string()));
            }
        }
    } else {
        if sources.len() > 1 {
            eprintln!("Error: target '{}' is not a directory", dest);
            process::exit(1);
        }
        tasks.push((sources[0].clone(), dest));
    }

    let mut conflicts = Vec::new();
    for t in &tasks {
        if Path::new(&t.1).exists() {
            conflicts.push(t.1.clone());
        }
    }

    if !conflicts.is_empty() && !force {
        eprintln!("Cannot overwrite existing file(s): {}", conflicts.join(", "));
        eprintln!("Use -f or --force to overwrite.");
        process::exit(1);
    }

    for task in tasks {
        let src_path = Path::new(&task.0);
        if !src_path.exists() {
            eprintln!("Error: Source file '{}' does not exist.", task.0);
            process::exit(1);
        }

        let content = match fs::read_to_string(src_path) {
            Ok(c) => c,
            Err(e) => {
                eprintln!("Error reading file '{}': {}", task.0, e);
                process::exit(1);
            }
        };

        let new_content = apply_replacements(&content, &replacements);
        let target_path = Path::new(&task.1);
        let overwritten = target_path.exists();

        if let Err(e) = fs::write(target_path, new_content) {
            eprintln!("Error writing file '{}': {}", task.1, e);
            process::exit(1);
        }

        if overwritten {
            println!("{} -> {} (overwrite)", task.0, task.1);
        } else {
            println!("{} -> {}", task.0, task.1);
        }
    }
}
