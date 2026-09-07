# frozen_string_literal: true

# tiny_clipboard.rb
# A single-file, zero-dependency clipboard library for Ruby.
# Perfect for vendoring, mimicking a C++ header-only library.
#
# This implementation references logic from the `clipboard` gem.
#
# MIT License
# Copyright (c) 2010-2023 Jan Lelis <janlelis@gmail.com> (for the original `clipboard` gem logic)
# Copyright (c) 2026 ongaeshi (for this single-file adaptation)
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

module TinyClipboard
  module_function

  def copy(text)
    case RUBY_PLATFORM
    when /mswin|mingw/
      windows_copy(text)
    when /darwin/
      mac_copy(text)
    when /linux|bsd/
      linux_copy(text)
    else
      raise "Unsupported platform: #{RUBY_PLATFORM}"
    end
  end

  def paste
    case RUBY_PLATFORM
    when /mswin|mingw/
      windows_paste
    when /darwin/
      mac_paste
    when /linux|bsd/
      linux_paste
    else
      raise "Unsupported platform: #{RUBY_PLATFORM}"
    end
  end

  # --- Windows Implementation ---
  if RUBY_PLATFORM =~ /mswin|mingw/
    require 'fiddle/import'

    module Win32
      extend Fiddle::Importer
      dlload 'user32', 'kernel32'

      extern 'int OpenClipboard(void*)'
      extern 'int CloseClipboard()'
      extern 'int EmptyClipboard()'
      extern 'void* SetClipboardData(int, void*)'
      extern 'void* GetClipboardData(int)'
      extern 'void* GlobalAlloc(int, int)'
      extern 'void* GlobalLock(void*)'
      extern 'int GlobalUnlock(void*)'
      extern 'int GlobalSize(void*)'

      CF_UNICODETEXT = 13
      GMEM_MOVEABLE = 0x0002
    end
  end

  def windows_copy(text)
    # Using nil for the window handle means the current task owns the clipboard
    return false unless Win32.OpenClipboard(nil) != 0

    begin
      Win32.EmptyClipboard()
      
      utf16 = text.encode(Encoding::UTF_16LE) + "\0".encode(Encoding::UTF_16LE)
      handle = Win32.GlobalAlloc(Win32::GMEM_MOVEABLE, utf16.bytesize)
      pointer = Win32.GlobalLock(handle)
      
      begin
        Fiddle::Pointer.new(pointer.to_i)[0, utf16.bytesize] = utf16
      ensure
        Win32.GlobalUnlock(handle)
      end

      Win32.SetClipboardData(Win32::CF_UNICODETEXT, handle)
    ensure
      Win32.CloseClipboard()
    end
    true
  end

  def windows_paste
    return "" unless Win32.OpenClipboard(nil) != 0

    begin
      handle = Win32.GetClipboardData(Win32::CF_UNICODETEXT)
      return "" if handle.to_i == 0

      pointer = Win32.GlobalLock(handle)
      begin
        size = Win32.GlobalSize(handle)
        data = Fiddle::Pointer.new(pointer.to_i)[0, size]
        
        # Remove trailing null characters and convert to UTF-8
        data.force_encoding(Encoding::UTF_16LE).encode(Encoding::UTF_8).sub(/\0.*\z/m, '')
      ensure
        Win32.GlobalUnlock(handle)
      end
    ensure
      Win32.CloseClipboard()
    end
  end

  # --- macOS Implementation ---
  def mac_copy(text)
    IO.popen('pbcopy', 'w') { |io| io.print text }
  end

  def mac_paste
    `pbpaste`
  end

  # --- Linux Implementation ---
  def linux_copy(text)
    if system('which wl-copy > /dev/null 2>&1')
      IO.popen('wl-copy', 'w') { |io| io.print text }
    elsif system('which xclip > /dev/null 2>&1')
      IO.popen('xclip -selection clipboard -i', 'w') { |io| io.print text }
    elsif system('which xsel > /dev/null 2>&1')
      IO.popen('xsel --clipboard --input', 'w') { |io| io.print text }
    else
      raise "Clipboard utility not found (requires wl-copy, xclip, or xsel)"
    end
  end

  def linux_paste
    if system('which wl-paste > /dev/null 2>&1')
      `wl-paste`
    elsif system('which xclip > /dev/null 2>&1')
      `xclip -selection clipboard -o`
    elsif system('which xsel > /dev/null 2>&1')
      `xsel --clipboard --output`
    else
      raise "Clipboard utility not found (requires wl-paste, xclip, or xsel)"
    end
  end
end
