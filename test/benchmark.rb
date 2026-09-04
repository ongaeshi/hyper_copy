require 'benchmark'
require 'fileutils'
require 'tmpdir'

puts "============================================="
puts " Building Binaries..."
puts "============================================="

# Create benchmark_bin directory
PROJECT_ROOT = File.expand_path('..', __dir__)
Dir.chdir(PROJECT_ROOT)
bin_dir = File.expand_path('benchmark_bin', PROJECT_ROOT)
FileUtils.rm_rf(bin_dir)
FileUtils.mkdir_p(bin_dir)

# Build Go
puts "[Go] Building..."
system("cd golang && go build -o ../benchmark_bin/hyper_copy_go.exe main.go")

# Build Rust
puts "[Rust] Building..."
system("cd rust && cargo build --release")
rust_exe = File.expand_path('rust/target/release/rust.exe', PROJECT_ROOT)
if File.exist?(rust_exe)
  FileUtils.cp(rust_exe, File.join(bin_dir, "hyper_copy_rust.exe"))
else
  # Cargo might name it hyper_copy.exe if we changed it, check just in case
  alt_rust_exe = File.expand_path('rust/target/release/hyper_copy.exe', PROJECT_ROOT)
  FileUtils.cp(alt_rust_exe, File.join(bin_dir, "hyper_copy_rust.exe")) if File.exist?(alt_rust_exe)
end

# Build C# Standard
puts "[C# Standard] Building..."
system("cd csharp/HyperCopy && dotnet publish -c Release -o ../../benchmark_bin/csharp_std")

# Build C# Self-Contained
puts "[C# Self-Contained] Building..."
system("cd csharp/HyperCopy && dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -o ../../benchmark_bin/csharp_sc")

puts "\n============================================="
puts " Generating Test Data..."
puts "============================================="

data_dir = File.expand_path('benchmark_data', PROJECT_ROOT)
src_dir = File.join(data_dir, 'src')
dst_dir = File.join(data_dir, 'dst')

FileUtils.rm_rf(data_dir)
FileUtils.mkdir_p(src_dir)

# Create 10 large dummy files
FILE_COUNT = 10
puts "Creating #{FILE_COUNT} files in #{src_dir}..."

FILE_COUNT.times do |i|
  content_chunk = <<~TEXT
    This is file #{i}.
    We have FooBar here.
    Also fooBar and FOOBAR and foobar.
    Let's test Japanese フー.
    And some other text barBar.
  TEXT
  
  File.open(File.join(src_dir, "TestFile_#{i}_FooBar.txt"), "w") do |f|
    10000.times { f.write(content_chunk) }
  end
end

puts "\n============================================="
puts " Running Benchmarks..."
puts "============================================="

targets = {
  "Ruby" => {
    cmd: "ruby #{File.expand_path('ruby/hyper_copy.rb', PROJECT_ROOT)}",
    bin_path: File.expand_path('ruby/hyper_copy.rb', PROJECT_ROOT) # N/A for size, we'll handle this
  },
  "Go" => {
    cmd: File.join(bin_dir, "hyper_copy_go.exe"),
    bin_path: File.join(bin_dir, "hyper_copy_go.exe")
  },
  "Rust" => {
    cmd: File.join(bin_dir, "hyper_copy_rust.exe"),
    bin_path: File.join(bin_dir, "hyper_copy_rust.exe")
  },
  "C# (Standard)" => {
    cmd: File.join(bin_dir, "csharp_std", "HyperCopy.exe"),
    bin_path: File.join(bin_dir, "csharp_std", "HyperCopy.exe")
  },
  "C# (Self-Contained)" => {
    cmd: File.join(bin_dir, "csharp_sc", "HyperCopy.exe"),
    bin_path: File.join(bin_dir, "csharp_sc", "HyperCopy.exe")
  }
}

results = []

def format_size(bytes)
  if bytes > 1024 * 1024
    format("%.2f MB", bytes.to_f / (1024 * 1024))
  elsif bytes > 1024
    format("%.2f KB", bytes.to_f / 1024)
  else
    "#{bytes} B"
  end
end

targets.each do |name, info|
  cmd_base = info[:cmd]
  bin_path = info[:bin_path]

  # Get binary size
  size_str = "-"
  if name == "Ruby"
    size_str = "N/A (Script)"
  elsif File.exist?(bin_path)
    size_str = format_size(File.size(bin_path))
  else
    size_str = "Error: Not found"
  end

  # Prepare destination
  FileUtils.rm_rf(dst_dir)
  FileUtils.mkdir_p(dst_dir)

  files = Dir.glob(File.join(src_dir, "*"))
  
  print "Running #{name}... "
  
  # Measure time
  time = Benchmark.realtime do
    if name == "Ruby"
      # For Ruby, we construct the command line differently to use 'ruby' executable
      cmd_args = ["ruby", cmd_base.split(' ', 2).last, "--from", "FooBar", "--to", "AaaBbb", *files, dst_dir]
      success = system(*cmd_args, out: IO::NULL, err: IO::NULL)
    else
      cmd_args = [cmd_base, "--from", "FooBar", "--to", "AaaBbb", *files, dst_dir]
      success = system(*cmd_args, out: IO::NULL, err: IO::NULL)
    end
    
    unless success
      puts "\nError running #{name}!"
    end
  end

  puts format("%.3f s", time)
  
  results << {
    name: name,
    time: time,
    size: size_str
  }
end

puts "\n============================================="
puts " Benchmark Results"
puts "============================================="

puts "| Language | Execution Time (s) | Binary Size |"
puts "| --- | --- | --- |"
results.sort_by { |r| r[:time] }.each do |r|
  puts "| #{r[:name]} | #{format("%.3f", r[:time])} s | #{r[:size]} |"
end

# Cleanup data dir if desired, but we can leave it for manual inspection
# FileUtils.rm_rf(data_dir)
