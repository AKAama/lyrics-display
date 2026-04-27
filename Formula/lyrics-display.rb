class LyricsDisplay < Formula
  desc "Show real-time Apple Music lyrics in the macOS menu bar"
  homepage "https://github.com/AKAama/lyrics-display"
  url "https://github.com/AKAama/lyrics-display/archive/refs/tags/v0.1.1.tar.gz"
  sha256 "8eaccb926da876838091174edd66ad3d5a63bd8132b388af42c321e83226b877"
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
