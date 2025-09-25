class Modfetch < Formula
  desc "Robust CLI/TUI downloader for LLM and Stable Diffusion assets"
  homepage "https://github.com/jxwalker/modfetch"
  version "0.5.0"
  on_macos do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_arm64"
      sha256 "ff756723babcba03ad7a737224d7528d62a8a92fa9857fbd0141f48181789327"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_amd64"
      sha256 "eca58c75467e08bc0a901383317011456b5e0f703cfb2fcd8306c512badfa964"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_arm64"
      sha256 "4076365128562cb1e6a0b9a7270490087e3d83a78c6d1f1d430ecb64aba0d785"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_amd64"
      sha256 "0894365fea64a3dabec178009b96a2818a43a6a27fa8642c83a3fe2bdbf7f972"
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

