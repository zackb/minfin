class Minfin < Formula
  desc "Personal finance app that syncs SimpleFIN accounts into local SQLite"
  homepage "https://github.com/zackb/minfin"
  url "https://github.com/zackb/minfin/archive/refs/tags/v0.1.6.tar.gz"
  sha256 "0100e18ba4639375fc79f8211834007a14cf476f2a7ba0be5f4e253f6b5f8df0"
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
