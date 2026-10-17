class Minfin < Formula
  desc "Personal finance app that syncs SimpleFIN accounts into local SQLite"
  homepage "https://github.com/zackb/minfin"
  url "https://github.com/zackb/minfin/archive/refs/tags/v0.1.7.tar.gz"
  sha256 "73ee1a9b3e6a0f3dd9c0acacdb879d648cc9d5f55683205effe393b6a3a2e5da"
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
