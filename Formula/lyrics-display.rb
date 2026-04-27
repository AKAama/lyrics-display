class LyricsDisplay < Formula
  desc "Show real-time Apple Music lyrics in the macOS menu bar"
  homepage "https://github.com/AKAama/lyrics-display"
  url "https://github.com/AKAama/lyrics-display/archive/refs/tags/v0.1.3.tar.gz"
  sha256 "7c1948872886b1d88b41590e785cde4dc51be2b42d4f3087fb56b7e91800cc49"
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
