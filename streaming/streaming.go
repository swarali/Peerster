package streaming

import (
	"os/exec"
	"fmt"
	"os"
)

func EncodeFile(file_name string) string {
	//var resolutions = map[string]string { "720p": "1280:720", "480p": "854:480",
	// "360p": "640:360", "240p": "426:240" }

	var resolutions = map[string]string { "720p": "1280:720" }

	for key, value := range(resolutions) {
		EncodeOneResolution(file_name, value, key)
	}

	return file_name + "720p.webm"
}

func EncodeOneResolution(file_name string, resolution string, name string) {
	var err error

	cmd := "ffmpeg"

	// VP9 args
	args := []string{ "-i", "files/" + file_name, "-c:v", "libvpx-vp9", "-vf", "scale=" + resolution, "-speed", "6",
		"-tile-columns", "4", "-frame-parallel", "1", "-threads", "8", "-static-thresh", "0", "-max-intra-rate", "300",
		"-deadline", "realtime", "-lag-in-frames", "0", "-error-resilient", "1", "files/" + file_name + name +".webm" }

	// VP8 args
	//args := []string{ "-i", "files/" + file_name, "-c:v", "libvpx", "-speed", "6", "-tile-columns", "4", "-frame-parallel", "1",
	//	"-static-thresh", "0", "-max-intra-rate", "300", "-deadline", "realtime", "-lag-in-frames", "0", "-error-resilient", "1",
	//	"files/" + file_name + ".webm" }

	// H264 args
	//args := []string{ "-i", "files/" + file_name, "-c:v", "libx264", "-speed", "6", "-tile-columns", "4", "-frame-parallel", "1",
	//	"-threads", "8", "-static-thresh", "0", "-max-intra-rate", "300", "-deadline", "realtime", "-lag-in-frames", "0",
	//	"-error-resilient", "1", "files/" + file_name + ".mp4" }

	if _, err = exec.Command(cmd, args...).Output(); err != nil {
		fmt.Fprintln(os.Stderr, "There was an error on encoding command: ", err)
		os.Exit(1)
	}

	fmt.Println("ENCODING SUCCEED")
}
