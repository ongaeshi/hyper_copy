#!/usr/bin/env ruby
# frozen_string_literal: true

require 'fileutils'

def preserve_case(match, to_str)
  return "" if to_str.nil? || to_str.empty?

  if match.include?('-') && to_str.include?('-')
    m_parts = match.split('-', -1)
    t_parts = to_str.split('-', -1)
    if m_parts.length == t_parts.length
      return m_parts.zip(t_parts).map { |m, t| preserve_case(m, t) }.join('-')
    end
  end

  if match.include?('_') && to_str.include?('_')
    m_parts = match.split('_', -1)
    t_parts = to_str.split('_', -1)
    if m_parts.length == t_parts.length
      return m_parts.zip(t_parts).map { |m, t| preserve_case(m, t) }.join('_')
    end
  end

  if match == match.upcase
    to_str.upcase
  elsif match == match.downcase
    to_str.downcase
  else
    if match[0] == match[0].downcase
      to_str[0].downcase + to_str[1..-1].to_s
    else
      to_str[0].upcase + to_str[1..-1].to_s
    end
  end
end

def apply_replacements(text, replacements)
  return text if replacements.empty?

  sorted_pairs = replacements.sort_by { |from, _| -from.length }
  regex = Regexp.union(sorted_pairs.map { |from, _| /#{Regexp.escape(from)}/i })

  text.gsub(regex) do |match|
    _, to_str = sorted_pairs.find { |from, _| match.casecmp?(from) }
    preserve_case(match, to_str)
  end
end

force = false
args = []
from_list = []
to_list = []

i = 0
while i < ARGV.length
  arg = ARGV[i]
  if arg == '-f' || arg == '--force'
    force = true
    i += 1
  elsif arg =~ /^--from(\d*)$/
    from_list << { suffix: $1, val: ARGV[i + 1] }
    i += 2
  elsif arg =~ /^--to(\d*)$/
    to_list << { suffix: $1, val: ARGV[i + 1] }
    i += 2
  else
    args << arg
    i += 1
  end
end

if args.length < 2
  warn "Usage: hyper_copy [options] <source...> <dest>"
  exit 1
end

replacements = []
from_list.each do |f|
  t = to_list.find { |t_item| t_item[:suffix] == f[:suffix] }
  if t
    replacements << [f[:val], t[:val]]
    to_list.delete(t)
  else
    warn "Error: Missing --to#{f[:suffix]} for --from#{f[:suffix]} #{f[:val]}"
    exit 1
  end
end

unless to_list.empty?
  warn "Error: Missing --from#{to_list.first[:suffix]} for --to#{to_list.first[:suffix]} #{to_list.first[:val]}"
  exit 1
end

sources = args[0...-1]
dest = args.last

tasks = []

if File.directory?(dest)
  sources.each do |src|
    new_basename = apply_replacements(File.basename(src), replacements)
    target_path = File.join(dest, new_basename)
    tasks << { src: src, dest: target_path }
  end
else
  if sources.length > 1
    warn "Error: target '#{dest}' is not a directory"
    exit 1
  end
  tasks << { src: sources.first, dest: dest }
end

# Pre-check for overwrites
conflicts = tasks.select { |t| File.exist?(t[:dest]) }
if conflicts.any? && !force
  conflict_files = conflicts.map { |t| t[:dest] }.join(", ")
  warn "Cannot overwrite existing file(s): #{conflict_files}"
  warn "Use -f or --force to overwrite."
  exit 1
end

# Perform actual copy/replacement
tasks.each do |task|
  src = task[:src]
  target = task[:dest]

  unless File.exist?(src)
    warn "Error: Source file '#{src}' does not exist."
    exit 1
  end

  # Using binmode to prevent Windows newline conversion issues during read/write if possible,
  # but text processing is fine with normal read. Force UTF-8 encoding.
  content = File.read(src, encoding: 'UTF-8')
  new_content = apply_replacements(content, replacements)

  overwritten = File.exist?(target)

  File.write(target, new_content, encoding: 'UTF-8')

  if overwritten
    puts "#{src} -> #{target} (overwrite)"
  else
    puts "#{src} -> #{target}"
  end
end
