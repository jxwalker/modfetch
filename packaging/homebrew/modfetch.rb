class Modfetch < Formula
  desc "Robust CLI/TUI downloader for LLM and Stable Diffusion assets"
  homepage "https://github.com/jxwalker/modfetch"
  version "0.8.0"
  on_macos do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_arm64"
      sha256 "9561c974403b7c24cdbbf7d61bdffee8cbf98c2e7eec0078dd0f7b04627348bc"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_darwin_amd64"
      sha256 "90b390ee9f1d9a9fa0ec712c1e5d7d07ce5efc2462cd3975ff1077a3c7acf2e1"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_arm64"
      sha256 "063f70619266d9a89ebf9178f412a50ae757977f9615d1fb8c3faa2e6771e150"
    end
    on_intel do
      url "https://github.com/jxwalker/modfetch/releases/download/v#{version}/modfetch_linux_amd64"
      sha256 "a7bbfb6ce482b91ab1a93eb3b3aa9618b1e61b943e078bf84befc0e3114c5e2a"
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
