class Modfetch < Formula
  desc "Robust CLI/TUI downloader for LLM and Stable Diffusion assets"
  homepage "https://github.com/jxwalker/modfetch"
  version "0.7.1"
  on_macos do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_arm64"
      sha256 "9b1eb0498197e10616c13996839aba4648800f4ac9d6ed8f17bdc9c4a54ca464"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_amd64"
      sha256 "174f34693fcae4476633d3c670a8f3366f3002baa15ea1ad6882d3f25ede1e6b"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_arm64"
      sha256 "893e51802932381bc14d472d94002813c491c5170f14e50573e0b30cf873c868"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_amd64"
      sha256 "c7db15650986c32cf4b1550699e624a929e21b926194a2523896fbacec95276c"
    end
  end

  def install
    if Hardware::CPU.arm? && OS.mac?
      bin.install "modfetch_darwin_arm64" => "modfetch"
    elsif Hardware::CPU.intel? && OS.mac?
      bin.install "modfetch_darwin_amd64" => "modfetch"
    elsif Hardware::CPU.arm? && OS.linux?
      bin.install "modfetch_linux_arm64" => "modfetch"
    else
      bin.install "modfetch_linux_amd64" => "modfetch"
    end
  end

  test do
    system "#{bin}/modfetch", "version"
  end
end
