class LyricsDisplay < Formula
  desc "Show real-time Apple Music lyrics in the macOS menu bar"
  homepage "https://github.com/AKAama/lyrics-display"
  url "https://github.com/AKAama/lyrics-display/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "431af11cb0f5fbe17a16a386355e6b4708d6d652cd82e20e1a68d9fa3710a20a"
  license "MIT"

  depends_on "go" => :build
  depends_on macos: :ventura

  def install
    ldflags = %W[
      -s
      -w
      -X main.version=#{version}
    ]

    system "go", "build", *std_go_args(ldflags: ldflags), "."
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/lyrics-display --version")
  end
end
