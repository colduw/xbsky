package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"main/internal/types"
)

func GenMosaic(w http.ResponseWriter, r *http.Request, images types.APIImages) {
	switch len(images) {
	case 0:
		ErrorPage(w, "genMosaic: No images")
		return
	case 1:
		http.Redirect(w, r, images[0].FullSize, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")

	var args []string
	var avgWidth int
	for _, k := range images {
		args = append(args, "-i", k.FullSize)
		avgWidth += int(k.AspectRatio.Width)
	}

	avgWidth /= len(images)

	var filterComplex strings.Builder
	for i := range images {
		fmt.Fprintf(&filterComplex, "[%d:v]scale=%d:-2[m%d];", i, avgWidth, i)
	}

	for i := range images {
		fmt.Fprintf(&filterComplex, "[m%d]", i)
	}
	fmt.Fprintf(&filterComplex, "hstack=inputs=%d", len(images))

	args = append(args, "-filter_complex", filterComplex.String(), "-f", "image2pipe", "-c:v", "mjpeg", "pipe:1")

	//nolint:gosec // This is just ffmpeg, with the only external values being k.FullSize, which is from the API
	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
	cmd.Stdout = w

	if runErr := cmd.Run(); runErr != nil {
		http.Error(w, "genMosaic: Failed to run", http.StatusInternalServerError)
		return
	}
}
