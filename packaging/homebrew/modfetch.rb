class Modfetch < Formula
  desc "Robust CLI/TUI downloader for LLM and Stable Diffusion assets"
  homepage "https://github.com/yourorg/modfetch"
  version "0.0.0"
  on_macos do
    on_arm do
      url "https://github.com/yourorg/modfetch/releases/download/v#{version}/modfetch_darwin_arm64"
      sha256 "REPLACE_WITH_SHA256"
    end
    on_intel do
      url "https://github.com/yourorg/modfetch/releases/download/v#{version}/modfetch_darwin_amd64"
      sha256 "REPLACE_WITH_SHA256"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/yourorg/modfetch/releases/download/v#{version}/modfetch_linux_arm64"
      sha256 "REPLACE_WITH_SHA256"
    end
    on_intel do
      url "https://github.com/yourorg/modfetch/releases/download/v#{version}/modfetch_linux_amd64"
      sha256 "REPLACE_WITH_SHA256"
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

