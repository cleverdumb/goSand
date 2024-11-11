package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"runtime"

	_ "image/png"
	"os"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// vertex shader source code
var vertexShaderSource = `
#version 410
in vec3 position;
in vec2 texCoord;
out vec2 TexCoord;

void main() {
    gl_Position = vec4(position, 1.0);
    TexCoord = texCoord;
}
` + "\x00"

// fragment shader source code
var fragmentShaderSource = `
#version 410
in vec2 TexCoord;
out vec4 color;
uniform sampler2D ourTexture;

void main() {
    color = texture(ourTexture, TexCoord);
}
` + "\x00"

const (
	gw   = 10
	gh   = 10
	scrW = 800
	scrH = 800
	bw   = scrW / gw
	bh   = scrH / gh
)

var square = []float32{
	-1, 1, 0, 0, 0,
	-1, 1 - float32(bh)/float32(scrH)*2, 0, 0, 1,
	-1 + float32(bw)/float32(scrW)*2, 1 - float32(bh)/float32(scrH)*2, 0, 1, 1,

	-1, 1, 0, 0, 0,
	-1 + float32(bw)/float32(scrW)*2, 1, 0, 1, 0,
	-1 + float32(bw)/float32(scrW)*2, 1 - float32(bh)/float32(scrH)*2, 0, 1, 1,
}

// var square = []float32{
// 	-1, 1, 0, 0, 0,
// 	-1, 0.8, 0, 0, 1,
// 	-0.8, 0.8, 0, 1, 1,

// 	-1, 1, 0, 0, 0,
// 	-0.8, 1, 0, 1, 0,
// 	-0.8, 0.8, 0, 1, 1,
// }

var program uint32

func init() {
	// This is needed to properly initialize OpenGL on macOS.
	runtime.LockOSThread()
}

func main() {
	// log.Println(square)
	if err := glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(scrW, scrH, "Sand", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		panic(err)
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		panic(err)
	}

	program = gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	sandTex, err := loadTexture("./sandTex.png")
	if err != nil {
		log.Fatalln("Failed to load texture sandTex")
	}

	dirtTex, err := loadTexture("./dirtTex.png")
	if err != nil {
		log.Fatalln("Failed to load texture dirtTex")
	}

	grid := make([][]*cell, gh)
	for yi := 0; yi < gh; yi++ {
		for xi := 0; xi < gw; xi++ {
			var c *cell
			if xi%2 == 0 {
				c = newCell(xi, yi, sandTex)
			} else {
				c = newCell(xi, yi, dirtTex)
			}
			grid[yi] = append(grid[yi], c)
		}
	}

	for !window.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		gl.UseProgram(program)

		for _, x := range grid {
			for _, y := range x {
				gl.BindTexture(gl.TEXTURE_2D, y.tex)
				gl.BindVertexArray(y.vao)
				gl.DrawArrays(gl.TRIANGLES, 0, int32(len(square)/3))
			}
		}

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func makeVao(points []float32) uint32 {
	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, 4*len(points), gl.Ptr(points), gl.STATIC_DRAW)

	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	// gl.EnableVertexAttribArray(0)
	// gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	// gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 0, nil)

	posAttrib := uint32(gl.GetAttribLocation(program, gl.Str("position\x00")))
	gl.EnableVertexAttribArray(posAttrib)
	gl.VertexAttribPointer(posAttrib, 3, gl.FLOAT, false, 5*4, nil)

	texCoordAttrib := uint32(gl.GetAttribLocation(program, gl.Str("texCoord\x00")))
	gl.EnableVertexAttribArray(texCoordAttrib)
	gl.VertexAttribPointer(texCoordAttrib, 2, gl.FLOAT, false, 5*4, unsafe.Pointer(uintptr(3*4)))

	return vao
}

type cell struct {
	x   int
	y   int
	t   int
	tex uint32

	vao uint32
}

func newCell(x int, y int, tex uint32) *cell {
	points := make([]float32, len(square))
	copy(points, square)

	for i := range points {
		switch i % 5 {
		case 0:
			points[i] += float32(bw) / float32(scrW) * 2 * float32(x)
		case 1:
			points[i] -= float32(bw) / float32(scrW) * 2 * float32(y)
		default:
			continue
		}
	}

	return &cell{
		x:   x,
		y:   y,
		t:   1,
		tex: tex,

		vao: makeVao(points),
	}
}

func loadTexture(name string) (uint32, error) {
	imgFile, err := os.Open(name)
	if err != nil {
		return 0, err
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(rgba.Rect.Size().X), int32(rgba.Rect.Size().Y), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	return texture, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := make([]byte, logLength+1)
		gl.GetShaderInfoLog(shader, logLength, nil, &log[0])

		return 0, fmt.Errorf("failed to compile %v: %v", source, string(log))
	}

	return shader, nil
}
