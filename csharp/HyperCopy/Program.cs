using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;

namespace HyperCopy
{
    class Program
    {
        static string RubyCapitalize(string s)
        {
            if (string.IsNullOrEmpty(s)) return s;
            var chars = s.ToCharArray();
            var first = char.ToUpper(chars[0]).ToString();
            var rest = new string(chars.Skip(1).Select(char.ToLower).ToArray());
            return first + rest;
        }

        static string PreserveCase(string match, string toStr)
        {
            if (string.IsNullOrEmpty(toStr)) return "";

            var matchUpper = match.ToUpper();
            var matchLower = match.ToLower();
            var matchCap = RubyCapitalize(match);

            if (match == matchUpper) return toStr.ToUpper();
            if (match == matchLower) return toStr.ToLower();
            if (match == matchCap) return RubyCapitalize(toStr);

            var firstMatch = match[0];
            var firstTo = toStr[0];
            var restTo = toStr.Substring(1);

            if (firstMatch == char.ToLower(firstMatch))
            {
                return char.ToLower(firstTo) + restTo;
            }
            else
            {
                return char.ToUpper(firstTo) + restTo;
            }
        }

        static string ApplyReplacements(string text, List<(string From, string To)> replacements)
        {
            if (replacements.Count == 0) return text;

            var sortedPairs = replacements.OrderByDescending(x => x.From.Length).ToList();
            var patternParts = sortedPairs.Select(x => Regex.Escape(x.From));
            var pattern = "(" + string.Join("|", patternParts) + ")";
            var regex = new Regex(pattern, RegexOptions.IgnoreCase);

            return regex.Replace(text, matchObj =>
            {
                var matchVal = matchObj.Value;
                var toStr = sortedPairs.First(x => string.Equals(x.From, matchVal, StringComparison.OrdinalIgnoreCase)).To;
                return PreserveCase(matchVal, toStr);
            });
        }

        static void Main(string[] args)
        {
            bool force = false;
            var argsList = new List<string>();
            var fromList = new List<(string Suffix, string Val)>();
            var toList = new List<(string Suffix, string Val)>();

            var fromRegex = new Regex(@"^--from(\d*)$");
            var toRegex = new Regex(@"^--to(\d*)$");

            for (int i = 0; i < args.Length; i++)
            {
                var arg = args[i];
                if (arg == "-f" || arg == "--force")
                {
                    force = true;
                }
                else if (fromRegex.IsMatch(arg))
                {
                    var match = fromRegex.Match(arg);
                    if (i + 1 < args.Length)
                    {
                        fromList.Add((match.Groups[1].Value, args[i + 1]));
                        i++;
                    }
                    else
                    {
                        Console.Error.WriteLine($"Error: Missing value for {arg}");
                        Environment.Exit(1);
                    }
                }
                else if (toRegex.IsMatch(arg))
                {
                    var match = toRegex.Match(arg);
                    if (i + 1 < args.Length)
                    {
                        toList.Add((match.Groups[1].Value, args[i + 1]));
                        i++;
                    }
                    else
                    {
                        Console.Error.WriteLine($"Error: Missing value for {arg}");
                        Environment.Exit(1);
                    }
                }
                else
                {
                    argsList.Add(arg);
                }
            }

            if (argsList.Count < 2)
            {
                Console.Error.WriteLine("Usage: hyper_copy [options] <source...> <dest>");
                Environment.Exit(1);
            }

            var replacements = new List<(string From, string To)>();
            foreach (var f in fromList)
            {
                var tIndex = toList.FindIndex(x => x.Suffix == f.Suffix);
                if (tIndex >= 0)
                {
                    replacements.Add((f.Val, toList[tIndex].Val));
                    toList.RemoveAt(tIndex);
                }
                else
                {
                    Console.Error.WriteLine($"Error: Missing --to{f.Suffix} for --from{f.Suffix} {f.Val}");
                    Environment.Exit(1);
                }
            }

            if (toList.Count > 0)
            {
                Console.Error.WriteLine($"Error: Missing --from{toList[0].Suffix} for --to{toList[0].Suffix} {toList[0].Val}");
                Environment.Exit(1);
            }

            var sources = argsList.Take(argsList.Count - 1).ToList();
            var dest = argsList.Last();

            var tasks = new List<(string Src, string Dest)>();

            if (Directory.Exists(dest))
            {
                foreach (var src in sources)
                {
                    var newBasename = ApplyReplacements(Path.GetFileName(src), replacements);
                    var targetPath = Path.Combine(dest, newBasename);
                    tasks.Add((src, targetPath));
                }
            }
            else
            {
                if (sources.Count > 1)
                {
                    Console.Error.WriteLine($"Error: target '{dest}' is not a directory");
                    Environment.Exit(1);
                }
                tasks.Add((sources[0], dest));
            }

            var conflicts = tasks.Where(t => File.Exists(t.Dest)).ToList();
            if (conflicts.Any() && !force)
            {
                var conflictFiles = string.Join(", ", conflicts.Select(t => t.Dest));
                Console.Error.WriteLine($"Cannot overwrite existing file(s): {conflictFiles}");
                Console.Error.WriteLine("Use -f or --force to overwrite.");
                Environment.Exit(1);
            }

            foreach (var task in tasks)
            {
                if (!File.Exists(task.Src))
                {
                    Console.Error.WriteLine($"Error: Source file '{task.Src}' does not exist.");
                    Environment.Exit(1);
                }

                var content = File.ReadAllText(task.Src, new UTF8Encoding(false));
                var newContent = ApplyReplacements(content, replacements);
                var overwritten = File.Exists(task.Dest);

                File.WriteAllText(task.Dest, newContent, new UTF8Encoding(false));

                if (overwritten)
                {
                    Console.WriteLine($"{task.Src} -> {task.Dest} (overwrite)");
                }
                else
                {
                    Console.WriteLine($"{task.Src} -> {task.Dest}");
                }
            }
        }
    }
}
