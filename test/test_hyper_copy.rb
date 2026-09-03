require 'minitest/autorun'
require 'fileutils'
require 'tmpdir'

module HyperCopyTestModule
  def setup
    @tmpdir = Dir.mktmpdir
    FileUtils.cp(File.expand_path('Foo.cs', __dir__), @tmpdir)
    FileUtils.cp(File.expand_path('Bar.cs', __dir__), @tmpdir)
    @original_dir = Dir.pwd
    Dir.chdir(@tmpdir)
  end

  def teardown
    Dir.chdir(@original_dir)
    FileUtils.remove_entry(@tmpdir)
  end

  def run_cmd(*args, fail_on_error: true)
    cmd = "#{self.class.command} #{args.join(' ')}"
    output = `#{cmd} 2>&1`
    result = $?.success?
    if fail_on_error && !result
      flunk "Command failed: #{cmd}\nOutput: #{output}"
    end
    [result, output]
  end

  def test_basic_replace
    run_cmd("--from", "FooBar", "--to", "AaaBbb", "Foo.cs", "Aaa.cs")
    assert File.exist?("Aaa.cs")
    content = File.read("Aaa.cs")
    assert_includes content, "AaaBbb"
    assert_includes content, "aaaBbb"
    assert_includes content, "AAABBB"
    refute_includes content, "FooBar"
  end

  def test_multiple_replace
    File.write("TestMultiple.txt", "FooBar and フー and barBar")
    run_cmd("--from", "FooBar", "--to", "AaaBbb", "--from2", "フー", "--to2", "バー", "--from3", "barBar", "--to3", "cccDdd", "TestMultiple.txt", "Out.txt")
    content = File.read("Out.txt")
    assert_equal "AaaBbb and バー and cccDdd", content
  end

  def test_directory_copy
    Dir.mkdir("outdir")
    run_cmd("--from", "Foo", "--to", "Aaa", "Foo.cs", "Bar.cs", "outdir")
    
    assert File.exist?("outdir/Aaa.cs")
    assert File.exist?("outdir/Bar.cs")
    
    aaa_content = File.read("outdir/Aaa.cs")
    assert_includes aaa_content, "AaaBar"
    
    bar_content = File.read("outdir/Bar.cs")
    assert_includes bar_content, "BarBar"
  end

  def test_overwrite_fails
    File.write("Aaa.cs", "Existing")
    result, output = run_cmd("--from", "FooBar", "--to", "AaaBbb", "Foo.cs", "Aaa.cs", fail_on_error: false)
    refute result, "Command should fail when trying to overwrite existing file without -f"
    assert_equal "Existing", File.read("Aaa.cs")
  end

  def test_force_overwrite
    File.write("Aaa.cs", "Existing")
    run_cmd("-f", "--from", "FooBar", "--to", "AaaBbb", "Foo.cs", "Aaa.cs")
    content = File.read("Aaa.cs")
    assert_includes content, "AaaBbb"
    refute_equal "Existing", content
  end
end

COMMANDS = {
  ruby: "ruby #{File.expand_path('../ruby/hyper_copy.rb', __dir__)}",
  golang: "go run #{File.expand_path('../golang/main.go', __dir__)}",
  rust: "cargo run --quiet --manifest-path #{File.expand_path('../rust/Cargo.toml', __dir__)} --",
  csharp: "dotnet run --project #{File.expand_path('../csharp/HyperCopy/HyperCopy.csproj', __dir__)} --"
}

langs_to_test = if ENV['TEST_LANG']
                  ENV['TEST_LANG'].split(',').map(&:to_sym)
                else
                  COMMANDS.keys
                end

langs_to_test.each do |lang|
  next unless COMMANDS.key?(lang)
  
  cls = Class.new(Minitest::Test) do
    include HyperCopyTestModule
    class << self
      attr_accessor :command
    end
  end
  cls.command = COMMANDS[lang]
  Object.const_set("TestHyperCopy#{lang.to_s.capitalize}", cls)
end
