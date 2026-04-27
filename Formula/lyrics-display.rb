class LyricsDisplay < Formula
  desc "Show real-time Apple Music lyrics in the macOS menu bar"
  homepage "https://github.com/AKAama/lyrics-display"
  url "https://github.com/AKAama/lyrics-display/archive/refs/tags/v0.1.2.tar.gz"
  sha256 "68712250af700b7bfd68b82020a36dfdc92e91a5d11cb03e0ecabeea88edc07b"
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
