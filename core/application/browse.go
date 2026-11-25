package application

import (
	"context"
	"errors"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

func (a *App) LoadSections(ctx context.Context) (forum.SectionList, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.SectionList, error) {
		return a.forum.Sections(requestCtx)
	})
}

func (a *App) LoadSection(ctx context.Context, name string) (forum.Section, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return forum.Section{}, errors.New("分区标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Section, error) {
		section, err := a.forum.Section(requestCtx, name)
		if err != nil {
			return forum.Section{}, err
		}
		for i := range section.Subsections {
			reference := section.Subsections[i]
			if reference.Description != "" || reference.Name == "" {
				continue
			}
			full, err := a.forum.Section(requestCtx, string(reference.Name))
			if err == nil {
				section.Subsections[i] = full
			}
		}
		return section, nil
	})
}

// LoadSectionTree reads a section and recursively resolves each subsection
// under one captured session, so the UI receives a coherent browse snapshot.
func (a *App) LoadSectionTree(ctx context.Context, name string) (forum.Section, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return forum.Section{}, errors.New("分区标识不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Section, error) {
		return readSectionTree(requestCtx, a.forum, name, "", make(map[string]bool), false)
	})
}

type sectionReader interface {
	Section(context.Context, string) (forum.Section, error)
}

func readSectionTree(ctx context.Context, client sectionReader, name, parent string, seen map[string]bool, strict bool) (forum.Section, error) {
	if name == "" {
		return forum.Section{}, errors.New("分区标识不能为空")
	}
	if seen[name] {
		return forum.Section{Name: forum.Scalar(name), Parent: forum.Scalar(parent)}, nil
	}
	seen[name] = true
	section, err := client.Section(ctx, name)
	if err != nil {
		return forum.Section{}, err
	}
	if section.Name == "" {
		section.Name = forum.Scalar(name)
	}
	if section.Parent == "" && parent != "" {
		section.Parent = forum.Scalar(parent)
	}
	for i, reference := range section.Subsections {
		child, err := readSectionTree(ctx, client, string(reference.Name), string(section.Name), seen, strict)
		if err != nil {
			if strict {
				return forum.Section{}, err
			}
			continue
		}
		if child.Description == "" {
			child.Description = reference.Description
		}
		section.Subsections[i] = child
	}
	return section, nil
}

func (a *App) LoadBoard(ctx context.Context, name string, page forum.Page) (forum.Board, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return forum.Board{}, errors.New("版面名称不能为空")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Board, error) {
		return a.forum.Board(requestCtx, name, page)
	})
}

func (a *App) SearchBoards(ctx context.Context, keyword string) (forum.BoardList, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return forum.BoardList{}, errors.New("请输入版面关键词")
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.BoardList, error) {
		return a.forum.SearchBoards(requestCtx, keyword)
	})
}

func (a *App) LoadAllBoards(ctx context.Context) ([]forum.Board, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) ([]forum.Board, error) {
		roots, err := a.forum.Sections(requestCtx)
		if err != nil {
			return nil, err
		}
		seenSections := make(map[string]bool)
		seenBoards := make(map[string]bool)
		boards := make([]forum.Board, 0)
		var collect func(forum.Section)
		collect = func(section forum.Section) {
			for _, board := range section.Boards {
				if board.Name != "" && !seenBoards[board.Name] {
					seenBoards[board.Name] = true
					boards = append(boards, board)
				}
			}
			for _, child := range section.Subsections {
				collect(child)
			}
		}
		for _, root := range roots.Sections {
			section, err := readSectionTree(requestCtx, a.forum, string(root.Name), "", seenSections, true)
			if err != nil {
				return nil, err
			}
			collect(section)
		}
		return boards, nil
	})
}

func (a *App) LoadTopTen(ctx context.Context) (forum.Widget, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Widget, error) {
		return a.forum.TopTen(requestCtx)
	})
}

func (a *App) LoadRecommendations(ctx context.Context) (forum.Recommendations, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Recommendations, error) {
		return a.forum.Recommendations(requestCtx)
	})
}

func (a *App) LoadTimeline(ctx context.Context, page forum.Page) (forum.ArticleList, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.ArticleList, error) {
		return a.forum.FavorPosts(requestCtx, page)
	})
}

func (a *App) LoadFavorites(ctx context.Context, directory string) (forum.Favorite, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Favorite, error) {
		return a.forum.Favorites(requestCtx, strings.TrimSpace(directory))
	})
}

type favoriteBoardSnapshot struct {
	favorite forum.Favorite
	boards   []forum.Board
}

func (a *App) LoadFavoriteBoards(ctx context.Context, directory string) (forum.Favorite, []forum.Board, error) {
	snapshot, err := runBound(a, ctx, func(requestCtx context.Context, _ Credential) (favoriteBoardSnapshot, error) {
		favorite, err := a.forum.Favorites(requestCtx, strings.TrimSpace(directory))
		if err != nil {
			return favoriteBoardSnapshot{}, err
		}
		listed := flattenFavoriteBoardValues(favorite)
		boards := make([]forum.Board, 0, len(listed))
		seen := make(map[string]bool, len(listed))
		for _, board := range listed {
			if board.Name == "" || seen[board.Name] {
				continue
			}
			seen[board.Name] = true
			detail, detailErr := a.forum.Board(requestCtx, board.Name, forum.Page{Size: 1})
			if detailErr != nil || detail.Name == "" {
				continue
			}
			detail.IsFavorite = true
			if detail.Description == "" {
				detail.Description = board.Description
			}
			boards = append(boards, detail)
		}
		return favoriteBoardSnapshot{favorite: favorite, boards: boards}, nil
	})
	return snapshot.favorite, snapshot.boards, err
}

func flattenFavoriteBoardValues(favorite forum.Favorite) []forum.Board {
	out := append([]forum.Board(nil), favorite.Boards...)
	for _, section := range favorite.Sections {
		out = append(out, section.Boards...)
		for _, subsection := range section.Subsections {
			out = append(out, subsection.Boards...)
		}
	}
	for _, child := range favorite.SubFavorites {
		out = append(out, flattenFavoriteBoardValues(child)...)
	}
	return out
}

func (a *App) LoadCollection(ctx context.Context, id string, page forum.Page) (forum.Collection, error) {
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Collection, error) {
		return a.forum.Collection(requestCtx, strings.TrimSpace(id), page)
	})
}

func (a *App) SetFavorite(ctx context.Context, board, directory string, favorite bool) (forum.Favorite, error) {
	board = strings.TrimSpace(board)
	directory = strings.TrimSpace(directory)
	if board == "" {
		return forum.Favorite{}, errors.New("版面名称不能为空")
	}
	if directory == "" {
		directory = "0"
	}
	return runBound(a, ctx, func(requestCtx context.Context, _ Credential) (forum.Favorite, error) {
		if favorite {
			return a.forum.AddFavorite(requestCtx, board, directory)
		}
		return a.forum.RemoveFavorite(requestCtx, board, directory)
	})
}
