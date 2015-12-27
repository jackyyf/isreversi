package common

import (
	"sync"
	"fmt"
)

var d = [8][2]int {
	{0, 1},
	{1, 0},
	{0, -1},
	{-1, 0},
	{1, 1},
	{1, -1},
	{-1, -1},
	{-1, 1},

}

type Game struct {
	ID uint16
	Board [8][8]uint8 // 0 = empty, 1 = black, 2 = white
	Turn uint8 // 0 = game over, 1 = black, 2 = white
	Black, White *User
	Mutex *sync.Mutex
}

func NewGame() *Game {
	ret := new(Game)
	ret.Turn = 0
	ret.Mutex = new(sync.Mutex)
	return ret
}

func (g *Game) Get(x, y int) uint8 { // 3 = invalid
	if x < 0 || x > 7 || y < 0 || y > 7 {
		return 3
	}
	return g.Board[x][y]
}

func (g *Game) CanPlace(ux, uy, who uint8) bool {
	x := int(ux)
	y := int(uy)
	if g.Get(x, y) != 0 {
		return false
	}
	for dir := 0; dir < 8; dir ++ {
		if n := g.Get(x + d[dir][0], y + d[dir][1]); n == 0 || n == who || n == 3 {
			// Can not change color in this direction
			continue
		}
		tx := x + d[dir][0]
		ty := y + d[dir][1]
		for {
			tx += d[dir][0]
			ty += d[dir][1]
			if g.Get(tx, ty) == 3 || g.Get(tx, ty) == 0 {
				break
			}
			if g.Get(tx, ty) == who {
				// Can change color in this direction
				return true
			}
		}
	}
	return false
}

func (g *Game) Place(ux, uy, who uint8) bool {
	x := int(ux)
	y := int(uy)
	if !g.CanPlace(ux, uy, who) {
		return false
	}
	if g.Turn != who {
		return false
	}
	for dir := 0; dir < 8; dir ++ {
		if n := g.Get(x + d[dir][0], y + d[dir][1]); n == 0 || n == who || n == 3 {
			// Can not change color in this direction
			continue
		}
		tx := int(x)
		ty := int(y)
		for {
			tx += d[dir][0]
			ty += d[dir][1]
			if g.Get(tx, ty) == 3 || g.Get(tx, ty) == 0 {
				break
			}
			if g.Get(tx, ty) == who {
				for tx != x || ty != y {
					fmt.Printf("%d %d\n", tx, ty)
					g.Board[tx][ty] = who
					tx -= d[dir][0]
					ty -= d[dir][1]
				}
				g.Board[x][y] = who
				break
			}
		}
	}
	if g.CanMove(3 - who) {
		g.Turn = 3 - who
	} else if g.CanMove(who) {
		// Keep moving :)
	} else {
		// Game over, no one can move
		g.Turn = 0
	}
	go g.White.refreshBoard()
	go g.Black.refreshBoard()
	return true
}

func (g *Game) CanMove(who uint8) bool {
	for x := uint8(0); x < 8; x ++ {
		for y := uint8(0); y < 8; y ++ {
			if g.CanPlace(x, y, who) {
				return true
			}
		}
	}
	return false
}

func (g *Game) GameOver() bool {
	return g.Turn == 0
}

func (g *Game) Restart() {
	for i := 0; i < 8; i ++ {
		for j := 0; j < 8; j ++ {
			g.Board[i][j] = 0
		}
	}
	g.Board[3][3] = 1
	g.Board[3][4] = 2
	g.Board[4][3] = 2
	g.Board[4][4] = 1
	g.Turn = 1
}

func (g *Game) TryStart() bool {
	if g.Black == nil || g.White == nil {
		return false
	}
	return g.Black.Ready && g.White.Ready
}
