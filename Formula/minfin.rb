class Minfin < Formula
  desc "Personal finance app that syncs SimpleFIN accounts into local SQLite"
  homepage "https://github.com/zackb/minfin"
  url "https://github.com/zackb/minfin/archive/refs/tags/v0.1.4.tar.gz"
  sha256 "5ec9f2af238050db8b57cd63d9f29b0e64aea175243f365e40e42ec5d80ee124"
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
