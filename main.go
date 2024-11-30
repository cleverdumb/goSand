package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

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
	gw   = 100
	gh   = 100
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

var texMap = make(map[string]uint32)
var grid = make([][]*cell, gh)

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

	gl.UseProgram(program)

	sandTex, err := loadTexture("./sandTex.png")
	if err != nil {
		log.Fatalln("Failed to load texture sandTex")
	}
	texMap["sand"] = sandTex

	dirtTex, err := loadTexture("./dirtTex.png")
	if err != nil {
		log.Fatalln("Failed to load texture dirtTex")
	}
	texMap["dirt"] = dirtTex

	for yi := 0; yi < gh; yi++ {
		for xi := 0; xi < gw; xi++ {
			var c *cell
			// r := rand.IntN(2)
			// switch r % 3 {
			// case 0:
			// 	c = newCell(xi, yi, "sand")
			// case 1:
			// 	c = newCell(xi, yi, "empty")
			// case 2:
			// 	c = newCell(xi, yi, "empty")
			// }
			if yi < gh/2 {
				c = newCell(xi, yi, "sand")
			} else {
				c = newCell(xi, yi, "empty")
			}
			grid[yi] = append(grid[yi], c)
		}
	}

	// grid[3][3] = newCell(3, 3, "sand")
	// grid[4][3] = newCell(3, 4, "empty")

	// renderAll(window)

	// for _, x := range grid[3][3].updateSqr() {
	// 	execUpdateBlock(x)
	// }

	// mainCh := make(chan *updatePack)
	quitCh := make(chan uint8)

	for x := 0; x < 8; x++ {
		go updateThread(quitCh)
	}

	// go renderThread(window, quitCh)

	for !window.ShouldClose() {
		// s := time.Now()
		// pack := <-mainCh
		// execUpdateBlock(pack)

		renderAll(window)
		time.Sleep((1000 / 30) * time.Millisecond)
		// log.Println(time.Since(s))
	}

	quitCh <- uint8(1)
}

// func renderThread(window *glfw.Window, quit chan uint8) {
// out:
// 	for {
// 		select {
// 		case <-quit:
// 			break out
// 		default:
// 			renderAll(window)
// 			time.Sleep((1000 / 30) * time.Millisecond)
// 		}
// 	}
// }

func updateThread(quit chan uint8) {
out:
	for {
		select {
		case <-quit:
			break out
		default:
			// s := time.Now()
			rx, ry := rand.IntN(gw), rand.IntN(gh)
			for grid[ry][rx].picked {
				rx, ry = rand.IntN(gw), rand.IntN(gh)
			}
			grid[ry][rx].picked = true
			for _, x := range grid[ry][rx].updateSqr() {
				// ch <- x
				execUpdateBlock(x)
			}
			grid[ry][rx].picked = false
			// log.Println(time.Since(s))

			time.Sleep(10 * time.Nanosecond)
		}
	}
}

func execUpdateBlock(x *updatePack) {
	grid[x.y][x.x].mut.Lock()
	grid[x.y][x.x].t = x.t
	grid[x.y][x.x].mut.Unlock()
}

func renderAll(window *glfw.Window) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	for _, x := range grid {
		for _, y := range x {
			if y.t != "empty" {
				y.draw()
			}
		}
	}

	window.SwapBuffers()
	glfw.PollEvents()
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
	x int
	y int
	t string

	vao uint32

	mut sync.Mutex

	picked bool
}

func (c *cell) draw() {
	gl.BindTexture(gl.TEXTURE_2D, texMap[c.t])
	gl.BindVertexArray(c.vao)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(square)/3))
}

func newCell(x int, y int, t string) *cell {
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
		x: x,
		y: y,
		t: t,

		vao: makeVao(points),
	}
}

type coord struct {
	x int
	y int
}

type updatePack struct {
	x int
	y int
	t string
}

func getAround(x int, y int) []*coord {
	res := make([]*coord, 0)

	if x > 0 {
		if y > 0 {
			res = append(res, &coord{x: x - 1, y: y - 1})
		}
		res = append(res, &coord{x: x - 1, y: y})
		if y < gh-1 {
			res = append(res, &coord{x: x - 1, y: y + 1})
		}
	}

	if y > 0 {
		res = append(res, &coord{x: x, y: y - 1})
	}
	res = append(res, &coord{x: x, y: y})
	if y < gh-1 {
		res = append(res, &coord{x: x, y: y + 1})
	}

	if x < gw-1 {
		if y > 0 {
			res = append(res, &coord{x: x + 1, y: y - 1})
		}
		res = append(res, &coord{x: x + 1, y: y})
		if y < gh-1 {
			res = append(res, &coord{x: x + 1, y: y + 1})
		}
	}

	return res
}

func (c *cell) updateSqr() []*updatePack {
	res := make([]*updatePack, 0)
	around := getAround(c.x, c.y)
	for _, coord := range around {
		grid[coord.y][coord.x].mut.Lock()
	}
	switch c.t {
	case "sand":
		if c.y < gh-1 && grid[c.y+1][c.x].t == "empty" {
			res = append(res, &updatePack{
				x: c.x,
				y: c.y,
				t: "empty",
			},
				&updatePack{
					x: c.x,
					y: c.y + 1,
					t: "sand",
				})
		}
	default:
		break
	}

	for _, coord := range around {
		grid[coord.y][coord.x].mut.Unlock()
	}

	return res
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
