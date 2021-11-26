package gol

const alive = 255
const dead = 0

func mod(x, m int) int {
	return (x + m) % m
}

func CalculateNewState(task Task, result Result) Result{
	AliveNum := 0
	for y := 1; y <= task.Height; y++ {
		for x := 0; x < task.Width; x++ {
			//calculate neighbours
			neighbours := 0
			for i := -1; i <= 1; i++ {
				for j := -1; j <= 1; j++ {
					if i != 0 || j != 0 {
						if task.World[y+i][mod(x+j, task.Width)] == alive {
							neighbours++
						}
					}
				}
			}

			//calculate the next state of the cell
			if task.World[y][x] == alive {
				if neighbours < 2 || neighbours > 3 {
					result.NewWorld[y-1][x] = dead
				} else {
					result.NewWorld[y-1][x] = alive
					AliveNum++
				}
			} else {
				if neighbours == 3 {
					result.NewWorld[y-1][x] = alive
					AliveNum++
				} else {
					result.NewWorld[y-1][x] = dead
				}
			}
		}
	}
	result.AliveNum= AliveNum
	return result
}

