class Minfin < Formula
  desc "Personal finance app that syncs SimpleFIN accounts into local SQLite"
  homepage "https://github.com/zackb/minfin"
  url "https://github.com/zackb/minfin/archive/refs/tags/v0.1.2.tar.gz"
  sha256 "ce49f1925298b8f75da2f42bf7836dc0303209c519c404da08ee60f5822a77ab"
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
