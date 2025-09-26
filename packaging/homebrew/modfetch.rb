class Modfetch < Formula
  desc "Robust CLI/TUI downloader for LLM and Stable Diffusion assets"
  homepage "https://github.com/jxwalker/modfetch"
  version "0.5.2"
  on_macos do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_arm64"
      sha256 "1cda41590d96a08f255c6e230e0cc23e73491b488ea0df500f73258e593ddff6"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_amd64"
      sha256 "a53533b5202b450c32e76d1c872c824ca4d0b42dada0e9049799c81bb08eb4bd"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_arm64"
      sha256 "f504fc75c2d2bdf011f817ec5a8d2742649576ccc192f4e87476e4f17d3bed41"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_amd64"
      sha256 "a9394e97bffec284b3c65bd16a02e62191be5aa11b9997b9a5cbd2cd6dc10b84"
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

