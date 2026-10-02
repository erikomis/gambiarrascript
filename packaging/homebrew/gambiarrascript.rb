# Formula Homebrew do GambiarraScript (template).
#
# A versao e os sha256 (marcadores entre __ abaixo) sao preenchidos a cada
# release pelo scripts/release; o workflow .github/workflows/release.yml imprime esta formula ja preenchida
# no resumo do job (aba Summary da run), pronta pra copiar e colar.
#
# Como publicar o tap (uma vez):
#   1. crie o repo publico erikomis/homebrew-tap no GitHub
#   2. coloque este arquivo (preenchido) em Formula/gambiarrascript.rb e de push
#   3. o povo instala com:
#        brew install erikomis/tap/gambiarrascript
# A cada release nova: cole a formula do resumo do job por cima e de push.
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

  def install
    bin.install "gs"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gs --version")
  end
end
