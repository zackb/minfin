class Minfin < Formula
  desc "Personal finance app that syncs SimpleFIN accounts into local SQLite"
  homepage "https://github.com/zackb/minfin"
  url "https://github.com/zackb/minfin/archive/refs/tags/v0.1.5.tar.gz"
  sha256 "792ca10d2bbafcce5500b0a62eb2f145b7e54504f422c801d847f3c0228933ca"
  license "MIT"
  head "https://github.com/zackb/minfin.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"minfin-desktop"), "./cmd/minfin-desktop"
    system "go", "build", *std_go_args(output: bin/"minfin-tui"), "./cmd/minfin-tui"
  end

  test do
    assert_path_exists bin/"minfin-desktop"
    assert_path_exists bin/"minfin-tui"
  end
end
