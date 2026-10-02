# Formula Homebrew do GambiarraScript (template).
#
# A versao e os sha256 (marcadores entre __ abaixo) sao preenchidos a cada
# release pelo scripts/release, e o .github/workflows/release.yml da push da
# formula pronta (so a partir da linha `class`) em erikomis/homebrew-tap:
# Formula/gambiarrascript.rb, usando a deploy key do secret TAP_DEPLOY_KEY.
# Pre-release (tag com hifen) nao mexe no tap.
#
# O povo instala com:
#   brew install erikomis/tap/gambiarrascript
class Gambiarrascript < Formula
  desc "Linguagem de programacao em portugues, feita na base da gambiarra"
  homepage "https://erikomis.github.io/gambiarrascript/"
  license "MIT"
  version "__VERSAO__"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/erikomis/gambiarrascript/releases/download/v#{version}/gs_#{version}_darwin_arm64.tar.gz"
      sha256 "__SHA256_DARWIN_ARM64__"
    else
      url "https://github.com/erikomis/gambiarrascript/releases/download/v#{version}/gs_#{version}_darwin_amd64.tar.gz"
      sha256 "__SHA256_DARWIN_AMD64__"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/erikomis/gambiarrascript/releases/download/v#{version}/gs_#{version}_linux_arm64.tar.gz"
      sha256 "__SHA256_LINUX_ARM64__"
    else
      url "https://github.com/erikomis/gambiarrascript/releases/download/v#{version}/gs_#{version}_linux_amd64.tar.gz"
      sha256 "__SHA256_LINUX_AMD64__"
    end
  end

  conflicts_with "ghostscript", because: "both install a `gs` binary"

  def install
    bin.install "gs"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gs --version")
  end
end
